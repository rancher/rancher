package fleetcluster

import (
	"context"
	"encoding/json"
	"errors"

	jsonpatch "github.com/evanphx/json-patch"
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/rancher/wrangler/pkg/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

type handler struct {
	clusters mgmtcontrollers.ClusterClient
	apply    apply.Apply
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusters: clients.Mgmt.Cluster(),
		apply:    clients.Apply.WithCacheTypes(clients.Provisioning.Cluster()),
	}

	clients.Mgmt.Cluster().OnChange(ctx, "fleet-cluster-label", h.addLabel)
	mgmtcontrollers.RegisterClusterGeneratingHandler(ctx,
		clients.Mgmt.Cluster(),
		clients.Apply.
			WithCacheTypes(clients.Fleet.Cluster(),
				clients.Provisioning.Cluster()),
		"",
		"fleet-cluster",
		h.createCluster,
		nil,
	)

	relatedresource.WatchClusterScoped(ctx, "fleet-cluster-resolver", h.clusterToCluster,
		clients.Mgmt.Cluster(), clients.Provisioning.Cluster())
}

func (h *handler) clusterToCluster(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	owner, err := h.apply.FindOwner(obj)
	if err != nil {
		// ignore error
		return nil, nil
	}
	if c, ok := owner.(*v1.Cluster); ok {
		return []relatedresource.Key{{
			Namespace: c.Namespace,
			Name:      c.Name,
		}}, nil
	}
	return nil, nil
}

func (h *handler) addLabel(key string, cluster *mgmt.Cluster) (*mgmt.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	if cluster.Spec.Internal && cluster.Spec.FleetWorkspaceName == "" {
		newCluster := cluster.DeepCopy()
		newCluster.Spec.FleetWorkspaceName = "fleet-local"
		patch, err := generatePatch(cluster, newCluster)
		if err != nil {
			return cluster, err
		}
		return h.clusters.Patch(cluster.Name, types.MergePatchType, patch)
	} else if cluster.Spec.Internal {
		return cluster, nil
	}

	if cluster.Spec.FleetWorkspaceName == "" {
		def := settings.FleetDefaultWorkspaceName.Get()
		if def == "" {
			return cluster, nil
		}

		newCluster := cluster.DeepCopy()
		newCluster.Spec.FleetWorkspaceName = def
		patch, err := generatePatch(cluster, newCluster)
		if err != nil {
			return cluster, err
		}
		cluster, err = h.clusters.Patch(cluster.Name, types.MergePatchType, patch)
		if err != nil {
			return nil, err
		}
	}

	if cluster.Spec.FleetWorkspaceName == "" {
		return cluster, nil
	}

	return cluster, nil
}

func (h *handler) createCluster(mgmtCluster *mgmt.Cluster, status mgmt.ClusterStatus) ([]runtime.Object, mgmt.ClusterStatus, error) {
	if mgmtCluster.Spec.FleetWorkspaceName == "" {
		return nil, status, nil
	}

	if !mgmt.ClusterConditionReady.IsTrue(mgmtCluster) {
		return nil, status, generic.ErrSkip
	}

	var (
		secretName       = mgmtCluster.Name + "-kubeconfig"
		fleetClusterName = mgmtCluster.Name
		rClusterName     = mgmtCluster.Name
		createCluster    = true
		objs             []runtime.Object
	)

	if owningCluster, err := h.apply.FindOwner(mgmtCluster); errors.Is(err, apply.ErrOwnerNotFound) || errors.Is(err, apply.ErrNoInformerFound) {
	} else if err != nil {
		return nil, status, err
	} else if rCluster, ok := owningCluster.(*v1.Cluster); ok {
		if rCluster.Status.ClientSecretName == "" {
			return nil, status, generic.ErrSkip
		}
		createCluster = false
		fleetClusterName = rCluster.Name
		rClusterName = rCluster.Name
		secretName = rCluster.Status.ClientSecretName
	}

	labels := yaml.CleanAnnotationsForExport(mgmtCluster.Labels)
	labels["management.cattle.io/cluster-name"] = mgmtCluster.Name
	labels["metadata.name"] = rClusterName
	if errs := validation.IsValidLabelValue(mgmtCluster.Spec.DisplayName); len(errs) == 0 {
		labels["management.cattle.io/cluster-display-name"] = mgmtCluster.Spec.DisplayName
	}

	if createCluster {
		objs = append(objs, &v1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rClusterName,
				Namespace: mgmtCluster.Spec.FleetWorkspaceName,
				Labels:    labels,
			},
			Spec: v1.ClusterSpec{
				ReferencedConfig: &v1.ReferencedConfig{
					ManagementClusterName: mgmtCluster.Name,
				},
				AgentEnvVars:                         mgmtCluster.Spec.AgentEnvVars,
				DefaultPodSecurityPolicyTemplateName: mgmtCluster.Spec.DefaultPodSecurityPolicyTemplateName,
				DefaultClusterRoleForProjectMembers:  mgmtCluster.Spec.DefaultClusterRoleForProjectMembers,
				EnableNetworkPolicy:                  mgmtCluster.Spec.EnableNetworkPolicy,
			},
		})
	}

	objs = append(objs, &fleet.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleetClusterName,
			Namespace: mgmtCluster.Spec.FleetWorkspaceName,
			Labels:    labels,
		},
		Spec: fleet.ClusterSpec{
			KubeConfigSecret: secretName,
			AgentEnvVars:     mgmtCluster.Spec.AgentEnvVars,
		},
	})

	return objs, status, nil
}

func generatePatch(old, new *mgmt.Cluster) ([]byte, error) {
	oldData, err := json.Marshal(old)
	if err != nil {
		return nil, err
	}

	newData, err := json.Marshal(new)
	if err != nil {
		return nil, err
	}

	return jsonpatch.CreateMergePatch(oldData, newData)
}
