package provisioningcluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/lasso/pkg/dynamic"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/features"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/condition"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	byNodeInfra                       = "by-node-infra"
	restoreRKEConfigKubernetesVersion = "kubernetesVersion"
	restoreRKEConfigAll               = "all"
	restoreRKEConfigNone              = "none"
)

type handler struct {
	dynamic           *dynamic.Controller
	dynamicSchema     mgmtcontroller.DynamicSchemaCache
	clusterCache      rocontrollers.ClusterCache
	clusterController rocontrollers.ClusterController
	secretCache       corecontrollers.SecretCache
	secretClient      corecontrollers.SecretClient
	capiClusters      capicontrollers.ClusterCache
	mgmtClusterCache  mgmtcontroller.ClusterCache
	mgmtClusterClient mgmtcontroller.ClusterClient
	rkeControlPlane   rkecontroller.RKEControlPlaneCache
	etcdSnapshotCache rkecontroller.ETCDSnapshotCache
	capiMachineCache  capicontrollers.MachineCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		dynamic:           clients.Dynamic,
		secretCache:       clients.Core.Secret().Cache(),
		secretClient:      clients.Core.Secret(),
		clusterCache:      clients.Provisioning.Cluster().Cache(),
		clusterController: clients.Provisioning.Cluster(),
		capiClusters:      clients.CAPI.Cluster().Cache(),
		rkeControlPlane:   clients.RKE.RKEControlPlane().Cache(),
		etcdSnapshotCache: clients.RKE.ETCDSnapshot().Cache(),
		capiMachineCache:  clients.CAPI.Machine().Cache(),
	}

	if features.MCM.Enabled() {
		h.dynamicSchema = clients.Mgmt.DynamicSchema().Cache()
		h.mgmtClusterCache = clients.Mgmt.Cluster().Cache()
		h.mgmtClusterClient = clients.Mgmt.Cluster()
	}

	clients.Dynamic.OnChange(ctx, "rke-dynamic", matchRKENodeGroup, h.infraWatch)
	clients.Provisioning.Cluster().Cache().AddIndexer(byNodeInfra, byNodeInfraIndex)

	rocontrollers.RegisterClusterGeneratingHandler(ctx,
		clients.Provisioning.Cluster(),
		clients.Apply.
			// Because capi wants to own objects we don't set ownerreference with apply
			WithDynamicLookup().
			WithCacheTypes(
				clients.CAPI.Cluster(),
				clients.CAPI.MachineDeployment(),
				clients.CAPI.MachineHealthCheck(),
				clients.RKE.RKEControlPlane(),
				clients.RKE.RKECluster(),
				clients.RKE.RKEBootstrapTemplate(),
			),
		"RKECluster",
		"rke-cluster",
		h.OnRancherClusterChange,
		nil)

	relatedresource.Watch(ctx, "provisioning-cluster-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if cp, ok := obj.(*rkev1.RKEControlPlane); ok {
			return []relatedresource.Key{{
				Namespace: namespace,
				Name:      cp.Spec.ClusterName,
			}}, nil
		}
		return nil, nil
	}, clients.Provisioning.Cluster(), clients.RKE.RKEControlPlane())

	clients.Provisioning.Cluster().OnChange(ctx, "provisioning-cluster-change", h.OnChange)
	clients.Provisioning.Cluster().OnRemove(ctx, "rke-cluster-remove", h.OnRemove)
}

func byNodeInfraIndex(obj *rancherv1.Cluster) ([]string, error) {
	if obj.Status.ClusterName == "" || obj.Spec.RKEConfig == nil {
		return nil, nil
	}

	var result []string
	for _, np := range obj.Spec.RKEConfig.MachinePools {
		if np.NodeConfig == nil {
			continue
		}
		result = append(result, toInfraRefKey(*np.NodeConfig, obj.Namespace))
	}

	return result, nil
}

func toInfraRefKey(ref corev1.ObjectReference, namespace string) string {
	if ref.APIVersion == "" {
		ref.APIVersion = capr.DefaultMachineConfigAPIVersion
	}
	return fmt.Sprintf("%s/%s/%s/%s", ref.APIVersion, ref.Kind, namespace, ref.Name)
}

func matchRKENodeGroup(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == capr.DefaultMachineConfigAPIVersion &&
		strings.HasSuffix(gvk.Kind, "Config")
}

func (h *handler) infraWatch(obj runtime.Object) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	typeInfo, err := meta.TypeAccessor(obj)
	if err != nil {
		return nil, err
	}

	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	indexKey := toInfraRefKey(corev1.ObjectReference{
		Kind:       typeInfo.GetKind(),
		Namespace:  meta.GetNamespace(),
		Name:       meta.GetName(),
		APIVersion: typeInfo.GetAPIVersion(),
	}, meta.GetNamespace())
	clusters, err := h.clusterCache.GetByIndex(byNodeInfra, indexKey)
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters {
		h.clusterController.Enqueue(cluster.Namespace, cluster.Name)
	}

	return obj, nil
}

func (h *handler) OnChange(_ string, cluster *rancherv1.Cluster) (*rancherv1.Cluster, error) {
	if cluster == nil || !cluster.DeletionTimestamp.IsZero() || cluster.Spec.RKEConfig == nil {
		return cluster, nil
	}

	// the outer loop searches for machine pools without a populated DynamicSchemaSpec field
	for i, machinePool := range cluster.Spec.RKEConfig.MachinePools {
		var spec apimgmtv3.DynamicSchemaSpec
		if machinePool.DynamicSchemaSpec != "" && json.Unmarshal([]byte(machinePool.DynamicSchemaSpec), &spec) == nil {
			continue
		}
		if machinePool.NodeConfig == nil {
			continue
		}
		apiVersion := machinePool.NodeConfig.APIVersion
		if apiVersion != capr.DefaultMachineConfigAPIVersion && apiVersion != "" {
			continue
		}
		// if the field is empty or invalid, add to any machine pools that do not have it and update the cluster
		clusterCopy := cluster.DeepCopy()
		for j := i; j < len(cluster.Spec.RKEConfig.MachinePools); j++ {
			machinePool := cluster.Spec.RKEConfig.MachinePools[j]
			spec = apimgmtv3.DynamicSchemaSpec{}
			if machinePool.DynamicSchemaSpec != "" && json.Unmarshal([]byte(machinePool.DynamicSchemaSpec), &spec) == nil {
				continue
			}
			nodeConfig := machinePool.NodeConfig
			if nodeConfig == nil {
				return cluster, fmt.Errorf("machine pool node config must not be nil")
			}
			apiVersion := nodeConfig.APIVersion
			if apiVersion != capr.DefaultMachineConfigAPIVersion && apiVersion != "" {
				continue
			}
			ds, err := h.dynamicSchema.Get(strings.ToLower(nodeConfig.Kind))
			if err != nil {
				return cluster, err
			}
			specJSON, err := json.Marshal(ds.Spec)
			if err != nil {
				return cluster, err
			}
			clusterCopy.Spec.RKEConfig.MachinePools[j].DynamicSchemaSpec = string(specJSON)
		}
		return h.clusterController.Update(clusterCopy)
	}
	return cluster, nil
}

func (h *handler) findSnapshotClusterSpec(snapshotNamespace, snapshotName string) (*rancherv1.ClusterSpec, error) {
	snapshot, err := h.etcdSnapshotCache.Get(snapshotNamespace, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving etcdsnapshot %s/%s: %w", snapshotNamespace, snapshotName, err)
	}
	return capr.ParseSnapshotClusterSpecOrError(snapshot)
}

// reconcileClusterSpecEtcdRestore reconciles the cluster against the desiredSpec, but only sets fields that should be set
// during an etcd restore. It expects the cluster object to be writable without conflict (i.e. DeepCopy). It returns a bool which indicates
// whether the cluster was changed
func reconcileClusterSpecEtcdRestore(cluster *rancherv1.Cluster, desiredSpec rancherv1.ClusterSpec) bool {
	// don't overwrite/change the cluster spec for certain entries
	changed := false
	if !equality.Semantic.DeepEqual(cluster.Spec.RKEConfig.MachineGlobalConfig, desiredSpec.RKEConfig.MachineGlobalConfig) {
		changed = true
		cluster.Spec.RKEConfig.MachineGlobalConfig = desiredSpec.RKEConfig.MachineGlobalConfig
	}
	if !equality.Semantic.DeepEqual(cluster.Spec.RKEConfig.MachineSelectorConfig, desiredSpec.RKEConfig.MachineSelectorConfig) {
		changed = true
		cluster.Spec.RKEConfig.MachineSelectorConfig = desiredSpec.RKEConfig.MachineSelectorConfig
	}
	if !equality.Semantic.DeepEqual(cluster.Spec.RKEConfig.ChartValues, desiredSpec.RKEConfig.ChartValues) {
		changed = true
		cluster.Spec.RKEConfig.ChartValues = desiredSpec.RKEConfig.ChartValues
	}
	if !equality.Semantic.DeepEqual(cluster.Spec.RKEConfig.Registries, desiredSpec.RKEConfig.Registries) {
		changed = true
		cluster.Spec.RKEConfig.Registries = desiredSpec.RKEConfig.Registries
	}
	if !equality.Semantic.DeepEqual(cluster.Spec.RKEConfig.UpgradeStrategy, desiredSpec.RKEConfig.UpgradeStrategy) {
		changed = true
		cluster.Spec.RKEConfig.UpgradeStrategy = desiredSpec.RKEConfig.UpgradeStrategy
	}
	if cluster.Spec.RKEConfig.AdditionalManifest != desiredSpec.RKEConfig.AdditionalManifest {
		changed = true
		cluster.Spec.RKEConfig.AdditionalManifest = desiredSpec.RKEConfig.AdditionalManifest
	}
	if cluster.Spec.RKEConfig.Networking != desiredSpec.RKEConfig.Networking {
		changed = true
		cluster.Spec.RKEConfig.Networking = desiredSpec.RKEConfig.Networking
	}
	if cluster.Spec.KubernetesVersion != desiredSpec.KubernetesVersion {
		changed = true
		cluster.Spec.KubernetesVersion = desiredSpec.KubernetesVersion
	}
	if cluster.Spec.DefaultPodSecurityAdmissionConfigurationTemplateName != desiredSpec.DefaultPodSecurityAdmissionConfigurationTemplateName {
		changed = true
		cluster.Spec.DefaultPodSecurityAdmissionConfigurationTemplateName = desiredSpec.DefaultPodSecurityAdmissionConfigurationTemplateName
	}
	if !equality.Semantic.DeepEqual(cluster.Spec.ClusterAgentDeploymentCustomization, desiredSpec.ClusterAgentDeploymentCustomization) {
		changed = true
		cluster.Spec.ClusterAgentDeploymentCustomization = desiredSpec.ClusterAgentDeploymentCustomization
	}
	if !equality.Semantic.DeepEqual(cluster.Spec.FleetAgentDeploymentCustomization, desiredSpec.FleetAgentDeploymentCustomization) {
		changed = true
		cluster.Spec.FleetAgentDeploymentCustomization = desiredSpec.FleetAgentDeploymentCustomization
	}
	return changed
}

// OnRancherClusterChange is called when the `clusters.provisioning.cattle.io` object is changed and is responsible for generating runtime images for the purpose of performing reconciliation
func (h *handler) OnRancherClusterChange(obj *rancherv1.Cluster, status rancherv1.ClusterStatus) ([]runtime.Object, rancherv1.ClusterStatus, error) {
	if obj.Spec.RKEConfig == nil || obj.Status.ClusterName == "" || obj.DeletionTimestamp != nil {
		return nil, status, nil
	}

	if obj.Spec.KubernetesVersion == "" {
		return nil, status, fmt.Errorf("kubernetesVersion not set on %s/%s", obj.Namespace, obj.Name)
	}

	if len(obj.Finalizers) == 0 && obj.DeletionTimestamp.IsZero() {
		// If the cluster doesn't have any finalizers, then we don't apply any objects to ensure the finalizer can be put on the cluster.
		return nil, status, generic.ErrSkip
	}

	rkeCP, err := h.getRKEControlPlaneForCluster(obj)
	if err != nil {
		return nil, status, err
	}

	mgmtCluster, err := h.retrieveMgmtClusterFromCache(obj)
	if err != nil {
		// don't return because the management cluster condition updating should not be blocking
		logrus.Errorf("rkecluster %s/%s: error while retrieving management cluster from cache: %v", obj.Namespace, obj.Name, err)
	}

	// If the rkecontrolplane is not nil, we can check it to determine action items.
	if rkeCP != nil {
		// If EtcdSnapshotRestore is not nil, we need to check to see if we need to update the cluster object it.
		if obj.Spec.RKEConfig.ETCDSnapshotRestore != nil &&
			obj.Spec.RKEConfig.ETCDSnapshotRestore.Name != "" &&
			obj.Spec.RKEConfig.ETCDSnapshotRestore.RestoreRKEConfig != "" &&
			obj.Spec.RKEConfig.ETCDSnapshotRestore.RestoreRKEConfig != restoreRKEConfigNone {
			logrus.Debugf("rkecluster %s/%s: Reconciling rkeconfig against specified etcd restore snapshot metadata", obj.Namespace, obj.Name)
			if !equality.Semantic.DeepEqual(rkeCP.Status.ETCDSnapshotRestore, obj.Spec.RKEConfig.ETCDSnapshotRestore) {
				clusterSpec, err := h.findSnapshotClusterSpec(obj.Namespace, obj.Spec.RKEConfig.ETCDSnapshotRestore.Name)
				if err != nil {
					return nil, status, err
				}
				switch obj.Spec.RKEConfig.ETCDSnapshotRestore.RestoreRKEConfig {
				case restoreRKEConfigKubernetesVersion:
					if obj.Spec.KubernetesVersion != clusterSpec.KubernetesVersion {
						logrus.Infof("rkecluster %s/%s: restoring Kubernetes version from %s to %s for etcd snapshot restore (snapshot: %s)", obj.Namespace, obj.Name, obj.Spec.KubernetesVersion, clusterSpec.KubernetesVersion, obj.Spec.RKEConfig.ETCDSnapshotRestore.Name)
						obj = obj.DeepCopy()
						obj.Spec.KubernetesVersion = clusterSpec.KubernetesVersion
						_, err = h.clusterController.Update(obj)
						if err == nil {
							err = generic.ErrSkip // if update was successful, return ErrSkip waiting for caches to sync
						}
						return nil, status, err
					}
				case restoreRKEConfigAll:
					newCluster := obj.DeepCopy()
					if reconcileClusterSpecEtcdRestore(newCluster, *clusterSpec) {
						logrus.Infof("rkecluster %s/%s: restoring RKE config for etcd snapshot restore (snapshot: %s)", obj.Namespace, obj.Name, obj.Spec.RKEConfig.ETCDSnapshotRestore.Name)
						_, err = h.clusterController.Update(newCluster)
						if err == nil {
							err = generic.ErrSkip // if update was successful, return ErrSkip waiting for caches to sync
						}
						return nil, status, err
					}
				}
			}
		}

		reconcileCondition(&status, capr.Updated, rkeCP, capr.Ready)
		reconcileCondition(&status, capr.Provisioned, rkeCP, capr.Ready)

		// If the Stable condition is not true, then copy the Ready condition from the rkeControlPlane to the v1.Clusters object
		// Otherwise, use the v3 clusters Ready condition. Note that we use `IsTrue` here because `IsFalse` specifically looks
		// for `False`, and the condition may not be defined which would not match `IsFalse`.
		useRKEControlPlaneReadyStatus := !capr.Stable.IsTrue(rkeCP)
		if useRKEControlPlaneReadyStatus {
			reconcileCondition(&status, capr.Ready, rkeCP, capr.Ready)
		}
		if mgmtCluster != nil {
			if !useRKEControlPlaneReadyStatus {
				reconcileCondition(&status, capr.Ready, mgmtCluster, capr.Ready)
			}
			reconcileCondition(mgmtCluster, capr.Updated, rkeCP, capr.Ready)
			reconcileCondition(mgmtCluster, capr.Provisioned, rkeCP, capr.Provisioned) // This was originally set by checking machine provisioning, but now we simply set it to true.
			_, err := h.mgmtClusterClient.Update(mgmtCluster)
			if err != nil {
				return nil, status, err
			}
		}
	}

	objs, err := objects(obj, h.dynamic, h.dynamicSchema, h.secretCache)
	return objs, status, err
}

// getRKEControlPlaneForCluster retrieves the rkecontrolplane that corresponds to a provisioning cluster object.
// If it cannot retrieve the corresponding CAPI cluster (is not found), if the capi cluster controlplane ref is not
// an rkecontrolplane, or the rkecontrolplane object can't be found and the cluster is deleting, it returns nil, nil.
func (h *handler) getRKEControlPlaneForCluster(cluster *rancherv1.Cluster) (*rkev1.RKEControlPlane, error) {
	capiCluster, err := h.capiClusters.Get(cluster.Namespace, cluster.Name)
	if apierror.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if capiCluster.Spec.ControlPlaneRef == nil ||
		capiCluster.Spec.ControlPlaneRef.Kind != "RKEControlPlane" {
		return nil, nil
	}

	cp, err := h.rkeControlPlane.Get(capiCluster.Spec.ControlPlaneRef.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if apierror.IsNotFound(err) && cluster.DeletionTimestamp != nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return cp, nil
}

// retrieveMgmtClusterFromCache retrieves an editable copy of the v3.Clusters object from the mgmtCache
func (h *handler) retrieveMgmtClusterFromCache(cluster *rancherv1.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil {
		return nil, fmt.Errorf("cluster was nil")
	}
	if h.mgmtClusterCache == nil {
		return nil, fmt.Errorf("management cluster cache was nil")
	}
	mgmtCluster, err := h.mgmtClusterCache.Get(cluster.Status.ClusterName)
	if err != nil {
		return nil, err
	}
	return mgmtCluster.DeepCopy(), nil
}

// reconcileCondition accepts an object, a condition to reconcile on the object, and the status/reason/message to reconcile.
// It returns a boolean indicating whether the condition was changed.
func reconcileCondition(to interface{}, toCondition condition.Cond, from interface{}, fromCondition condition.Cond) bool {
	if fromCondition.GetStatus(from) != toCondition.GetStatus(to) || fromCondition.GetReason(from) != toCondition.GetReason(to) || fromCondition.GetMessage(from) != toCondition.GetMessage(to) {
		toCondition.SetStatus(to, fromCondition.GetStatus(from))
		toCondition.Reason(to, fromCondition.GetReason(from))
		toCondition.Message(to, fromCondition.GetMessage(from))
		return true
	}
	return false
}

func (h *handler) OnRemove(_ string, cluster *rancherv1.Cluster) (*rancherv1.Cluster, error) {
	if cluster == nil || cluster.Spec.RKEConfig == nil || cluster.Status.ClusterName == "" {
		return nil, nil
	}

	rkeCP, err := h.getRKEControlPlaneForCluster(cluster)
	if err != nil || rkeCP == nil {
		return cluster, err
	}

	// Check to see if the management cluster still exists. If it exists, copy the Removed condition from the
	// controlplane to the object. If it does not exist, allow the v1 Cluster object to be removed.
	mgmtCluster, err := h.retrieveMgmtClusterFromCache(cluster)
	if err != nil {
		if apierror.IsNotFound(err) {
			// go ahead and proceed with removal
			return cluster, nil
		}
		logrus.Errorf("rkecluster %s/%s: error retrieving management cluster during removal of cluster: %v", cluster.Namespace, cluster.Name, err)
	}
	if mgmtCluster != nil && reconcileCondition(mgmtCluster, capr.Removed, rkeCP, capr.Removed) {
		_, err = h.mgmtClusterClient.Update(mgmtCluster)
		if apierror.IsNotFound(err) {
			return cluster, nil
		} else if err != nil {
			return cluster, err
		}
	}

	status := cluster.Status.DeepCopy()
	if reconcileCondition(status, capr.Removed, rkeCP, capr.Removed) {
		cluster.Status = *status
		cluster, err = h.clusterController.UpdateStatus(cluster)
		if err != nil {
			return cluster, err
		}
	}
	return cluster, generic.ErrSkip
}
