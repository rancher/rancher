package provisioningcluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/lasso/pkg/dynamic"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/features"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
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
		ref.APIVersion = rke2.DefaultMachineConfigAPIVersion
	}
	return fmt.Sprintf("%s/%s/%s/%s", ref.APIVersion, ref.Kind, namespace, ref.Name)
}

func matchRKENodeGroup(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == rke2.DefaultMachineConfigAPIVersion &&
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

func (h *handler) findSnapshotClusterSpec(snapshotNamespace, snapshotName string) (*rancherv1.ClusterSpec, error) {
	snapshot, err := h.etcdSnapshotCache.Get(snapshotNamespace, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving etcdsnapshot %s/%s: %w", snapshotNamespace, snapshotName, err)
	}
	if snapshot.SnapshotFile.Metadata != "" {
		var md map[string]string
		b, err := base64.StdEncoding.DecodeString(snapshot.SnapshotFile.Metadata)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &md); err != nil {
			return nil, err
		}
		if v, ok := md["provisioning-cluster-spec"]; ok {
			return decompressClusterSpec(v)
		}
	}
	return nil, fmt.Errorf("unable to find and decode snapshot ClusterSpec for snapshot %s/%s", snapshotNamespace, snapshotName)
}

// reconcileClusterSpecEtcdRestore reconciles the cluster against the desiredSpec, but only sets fields that should be set
// during an etcd restore. It expects the cluster object to be writable without conflict (i.e. DeepCopy)
func reconcileClusterSpecEtcdRestore(cluster *rancherv1.Cluster, desiredSpec rancherv1.ClusterSpec) {
	// don't overwrite/change the cluster spec for certain entries
	cluster.Spec.RKEConfig.MachineGlobalConfig = desiredSpec.RKEConfig.MachineGlobalConfig
	cluster.Spec.RKEConfig.MachineSelectorConfig = desiredSpec.RKEConfig.MachineSelectorConfig
	cluster.Spec.RKEConfig.ChartValues = desiredSpec.RKEConfig.ChartValues
	cluster.Spec.RKEConfig.AdditionalManifest = desiredSpec.RKEConfig.AdditionalManifest
	cluster.Spec.KubernetesVersion = desiredSpec.KubernetesVersion
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
		return nil, status, nil
	}

	rkeCP, err := h.getRKEControlPlaneForCluster(obj)
	if err != nil {
		return nil, status, err
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
				clusterSpec, err := h.findSnapshotClusterSpec(obj.Namespace, fmt.Sprintf("%s-%s", obj.Name, obj.Spec.RKEConfig.ETCDSnapshotRestore.Name))
				if err != nil {
					return nil, status, err
				}
				switch obj.Spec.RKEConfig.ETCDSnapshotRestore.RestoreRKEConfig {
				case restoreRKEConfigKubernetesVersion:
					if obj.Spec.KubernetesVersion != clusterSpec.KubernetesVersion {
						logrus.Infof("rkecluster %s/%s: restoring Kubernetes version from %s to %s for etcd snapshot restore (snapshot: %s)", obj.Namespace, obj.Name, obj.Spec.KubernetesVersion, clusterSpec.KubernetesVersion, obj.Spec.RKEConfig.ETCDSnapshotRestore.Name)
						obj = obj.DeepCopy()
						obj.Spec.KubernetesVersion = clusterSpec.KubernetesVersion
						_, err := h.clusterController.Update(obj)
						if err == nil {
							err = generic.ErrSkip // if update was successful, return ErrSkip waiting for caches to sync
						}
						return nil, status, err
					}
				case restoreRKEConfigAll:
					newCluster := obj.DeepCopy()
					reconcileClusterSpecEtcdRestore(newCluster, *clusterSpec)
					if !equality.Semantic.DeepEqual(newCluster.Spec, clusterSpec) {
						logrus.Infof("rkecluster %s/%s: restoring RKE config for etcd snapshot restore (snapshot: %s)", obj.Namespace, obj.Name, obj.Spec.RKEConfig.ETCDSnapshotRestore.Name)
						_, err := h.clusterController.Update(newCluster)
						if err == nil {
							err = generic.ErrSkip // if update was successful, return ErrSkip waiting for caches to sync
						}
						return nil, status, err
					}
				}
			}
		}
		logrus.Debugf("rkecluster %s/%s: updating cluster provisioning status", obj.Namespace, obj.Name)
		status, err = h.updateClusterProvisioningStatus(obj, status, rkeCP)
		if err != nil && !apierror.IsNotFound(err) {
			return nil, status, err
		}
	}

	objs, err := objects(obj, h.dynamic, h.dynamicSchema, h.secretCache)
	return objs, status, err
}

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
	if apierror.IsNotFound(err) && cluster.DeletionTimestamp == nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return cp, nil
}

func (h *handler) updateClusterProvisioningStatus(cluster *rancherv1.Cluster, status rancherv1.ClusterStatus, cp *rkev1.RKEControlPlane) (rancherv1.ClusterStatus, error) {
	if cp == nil {
		return status, fmt.Errorf("error while updating cluster provisioning status - rkecontrolplane was nil")
	}
	if cluster.DeletionTimestamp != nil && h.mgmtClusterCache != nil {
		mgmtCluster, err := h.mgmtClusterCache.Get(cluster.Status.ClusterName)
		if err != nil {
			return status, err
		}

		message := rke2.Ready.GetMessage(cp)
		if (message == "" && rke2.Ready.GetMessage(mgmtCluster) != "") || strings.Contains(message, planner.ETCDRestoreMessage) {
			mgmtCluster = mgmtCluster.DeepCopy()

			rke2.Provisioned.SetStatus(mgmtCluster, rke2.Ready.GetStatus(cp))
			rke2.Provisioned.Reason(mgmtCluster, rke2.Ready.GetReason(cp))
			rke2.Provisioned.Message(mgmtCluster, message)

			_, err = h.mgmtClusterClient.Update(mgmtCluster)
			if err != nil {
				return status, err
			}
		}
	}

	clusterCondition := rke2.Provisioned
	cpCondition := rke2.Ready
	if !cluster.DeletionTimestamp.IsZero() {
		clusterCondition = rke2.Removed
		cpCondition = rke2.Removed
	}
	clusterCondition.SetStatus(&status, cpCondition.GetStatus(cp))
	clusterCondition.Reason(&status, cpCondition.GetReason(cp))
	clusterCondition.Message(&status, cpCondition.GetMessage(cp))

	return status, nil
}

func (h *handler) OnRemove(_ string, cluster *rancherv1.Cluster) (*rancherv1.Cluster, error) {
	if cluster == nil || cluster.Spec.RKEConfig == nil || cluster.Status.ClusterName == "" {
		return nil, nil
	}

	if _, err := h.capiClusters.Get(cluster.Namespace, cluster.Name); err != nil && apierror.IsNotFound(err) {
		return cluster, nil
	}

	rkeCP, err := h.getRKEControlPlaneForCluster(cluster)
	if err != nil {
		return cluster, err
	}

	status := *cluster.Status.DeepCopy()
	if rkeCP != nil {
		status, err = h.updateClusterProvisioningStatus(cluster, status, rkeCP)
		if apierror.IsNotFound(err) {
			return cluster, nil
		} else if err != nil {
			return cluster, err
		}
	}

	cluster.Status = status
	cluster, err = h.clusterController.UpdateStatus(cluster)
	if err != nil {
		return cluster, err
	}
	return cluster, generic.ErrSkip
}
