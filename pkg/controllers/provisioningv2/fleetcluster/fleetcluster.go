package fleetcluster

import (
	"context"
	"time"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	mgmtcluster "github.com/rancher/rancher/pkg/cluster"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	fleetcontrollers "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
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
	clusters          v3.ClusterClient
	clustersCache     v3.ClusterCache
	fleetClusters     fleetcontrollers.ClusterController
	apply             apply.Apply
	getPrivateRepoURL func(*provv1.Cluster, *apimgmtv3.Cluster) string
}

// Register registers the fleetcluster controller, which is responsible for creating fleet cluster objects.
// When fleet cluster objects are created, Fleet uses the object to deploy the fleet-agent into the cluster. Notably,
// the fleetcluster operates on both provisioning and management clusters in Rancher, by way of transformation logic
// in the provisioningcluster rke2 controller (a clusters.provisioning.cattle.io/v1 object is generated for every
// corresponding clusters.management.cattle.io/v3 object, if one does not already exist, and vice-versa)
func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusters:      clients.Mgmt.Cluster(),
		clustersCache: clients.Mgmt.Cluster().Cache(),
		fleetClusters: clients.Fleet.Cluster(),
		apply:         clients.Apply.WithCacheTypes(clients.Provisioning.Cluster()),
	}

	h.getPrivateRepoURL = func(cluster *provv1.Cluster, mgmtCluster *apimgmtv3.Cluster) string {
		if cluster.Spec.RKEConfig == nil {
			// If the RKEConfig is nil, we are likely dealing with
			// a legacy (v3/mgmt) cluster, and need to check the v3
			// cluster for the cluster level registry.
			return mgmtcluster.GetPrivateRegistryURL(mgmtCluster)
		}
		return image.GetPrivateRepoURLFromCluster(cluster)
	}

	rocontrollers.RegisterClusterGeneratingHandler(ctx,
		clients.Provisioning.Cluster(),
		clients.Apply.
			WithCacheTypes(clients.Fleet.Cluster(),
				clients.Fleet.ClusterGroup(),
				clients.Provisioning.Cluster()),
		"",
		"fleet-cluster",
		h.createCluster,
		nil,
	)

	clients.Mgmt.Cluster().OnChange(ctx, "fleet-cluster-assign", h.assignWorkspace)
	clients.Fleet.Cluster().OnChange(ctx, "fleet-local-agent-migration", h.ensureAgentMigrated)
}

func (h *handler) assignWorkspace(key string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	if cluster.Spec.Internal && cluster.Spec.FleetWorkspaceName == "" {
		newCluster := cluster.DeepCopy()
		newCluster.Spec.FleetWorkspaceName = fleetconst.ClustersLocalNamespace
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

func (h *handler) ensureAgentMigrated(key string, cluster *fleet.Cluster) (*fleet.Cluster, error) {
	if cluster != nil && cluster.Name == "local" && cluster.Namespace == fleetconst.ClustersLocalNamespace &&
		cluster.Spec.AgentNamespace == "" {
		// keep re-enqueueing until agentNamespace is set. This happens before the fleet
		// CRD is upgraded to include the new agentNamespace field
		h.fleetClusters.EnqueueAfter(cluster.Namespace, cluster.Name, 5*time.Second)
	}
	return cluster, nil
}

func (h *handler) createCluster(cluster *provv1.Cluster, status provv1.ClusterStatus) ([]runtime.Object, provv1.ClusterStatus, error) {
	if status.ClusterName == "" || status.ClientSecretName == "" {
		return nil, status, nil
	}

	mgmtCluster, err := h.clustersCache.Get(status.ClusterName)
	if err != nil {
		return nil, status, err
	}

	if !apimgmtv3.ClusterConditionReady.IsTrue(mgmtCluster) {
		return nil, status, generic.ErrSkip
	}

	// this removes any annotations containing "cattle.io" or starting with "kubectl.kubernetes.io"
	labels := yaml.CleanAnnotationsForExport(mgmtCluster.Labels)
	labels["management.cattle.io/cluster-name"] = mgmtCluster.Name
	if errs := validation.IsValidLabelValue(mgmtCluster.Spec.DisplayName); len(errs) == 0 {
		labels["management.cattle.io/cluster-display-name"] = mgmtCluster.Spec.DisplayName
	}

	agentNamespace := ""
	clientSecret := status.ClientSecretName

	objs := []runtime.Object{}
	if mgmtCluster.Spec.Internal {
		agentNamespace = fleetconst.ReleaseLocalNamespace
		// restore fleet's hardcoded name label for the local cluster
		labels["name"] = "local"
		// default cluster group, used if fleet bundle has no targets, uses hardcoded name label
		objs = append(objs, &fleet.ClusterGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: cluster.Namespace,
			},
			Spec: fleet.ClusterGroupSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": "local"},
				},
			},
		})
	}

	agentAffinity, err := mgmtcluster.GetFleetAgentAffinity(mgmtCluster)
	if err != nil {
		return nil, status, err
	}

	return append(objs, &fleet.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: fleet.ClusterSpec{
			KubeConfigSecret: clientSecret,
			AgentEnvVars:     mgmtCluster.Spec.AgentEnvVars,
			AgentNamespace:   agentNamespace,
			PrivateRepoURL:   h.getPrivateRepoURL(cluster, mgmtCluster),
			AgentTolerations: mgmtcluster.GetFleetAgentTolerations(mgmtCluster),
			AgentAffinity:    agentAffinity,
			AgentResources:   mgmtcluster.GetFleetAgentResourceRequirements(mgmtCluster),
		},
	}), status, nil
}
