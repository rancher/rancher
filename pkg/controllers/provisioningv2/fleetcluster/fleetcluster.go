package fleetcluster

import (
	"context"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
)

type handler struct {
	clusters      mgmtcontrollers.ClusterClient
	clustersCache mgmtcontrollers.ClusterCache
	apply         apply.Apply
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusters:      clients.Mgmt.Cluster(),
		clustersCache: clients.Mgmt.Cluster().Cache(),
		apply:         clients.Apply.WithCacheTypes(clients.Provisioning.Cluster()),
	}

	rocontrollers.RegisterClusterGeneratingHandler(ctx,
		clients.Provisioning.Cluster(),
		clients.Apply.
			WithCacheTypes(clients.Fleet.Cluster(),
				clients.Provisioning.Cluster()),
		"",
		"fleet-cluster",
		h.createCluster,
		nil,
	)

	clients.Mgmt.Cluster().OnChange(ctx, "fleet-cluster-assign", h.assignWorkspace)
}

func (h *handler) assignWorkspace(key string, cluster *mgmt.Cluster) (*mgmt.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	if cluster.Spec.Internal && cluster.Spec.FleetWorkspaceName == "" {
		newCluster := cluster.DeepCopy()
		newCluster.Spec.FleetWorkspaceName = "fleet-local"
		return h.clusters.Update(newCluster)
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
		return h.clusters.Update(newCluster)
	}

	return cluster, nil
}

func (h *handler) createCluster(cluster *v1.Cluster, status v1.ClusterStatus) ([]runtime.Object, v1.ClusterStatus, error) {
	if status.ClusterName == "" || status.ClientSecretName == "" {
		return nil, status, nil
	}

	mgmtCluster, err := h.clustersCache.Get(status.ClusterName)
	if err != nil {
		return nil, status, err
	}

	if !mgmt.ClusterConditionReady.IsTrue(mgmtCluster) {
		return nil, status, generic.ErrSkip
	}

	labels := yaml.CleanAnnotationsForExport(mgmtCluster.Labels)
	labels["management.cattle.io/cluster-name"] = mgmtCluster.Name
	if errs := validation.IsValidLabelValue(mgmtCluster.Spec.DisplayName); len(errs) == 0 {
		labels["management.cattle.io/cluster-display-name"] = mgmtCluster.Spec.DisplayName
	}

	return []runtime.Object{&fleet.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: fleet.ClusterSpec{
			KubeConfigSecret: status.ClientSecretName,
			AgentEnvVars:     mgmtCluster.Spec.AgentEnvVars,
		},
	}}, status, nil
}
