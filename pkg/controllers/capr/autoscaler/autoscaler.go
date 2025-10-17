package autoscaler

import (
	"context"
	"errors"
	"reflect"
	"time"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	fleetcontrollers "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v2provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// dynamicGetter defines the interface for the Get method from dynamic.Controller
type dynamicGetter interface {
	Get(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error)
}

type autoscalerHandler struct {
	capiClusterCache           v1beta1.ClusterCache
	capiMachineCache           v1beta1.MachineCache
	capiMachineDeploymentCache v1beta1.MachineDeploymentCache

	clusterClient v2provcontrollers.ClusterClient
	clusterCache  v2provcontrollers.ClusterCache

	globalRoleClient mgmtcontrollers.GlobalRoleClient
	globalRoleCache  mgmtcontrollers.GlobalRoleCache

	globalRoleBindingClient mgmtcontrollers.GlobalRoleBindingClient
	globalRoleBindingCache  mgmtcontrollers.GlobalRoleBindingCache

	userClient mgmtcontrollers.UserClient
	userCache  mgmtcontrollers.UserCache

	tokenClient mgmtcontrollers.TokenClient
	tokenCache  mgmtcontrollers.TokenCache

	secretClient wranglerv1.SecretClient
	secretCache  wranglerv1.SecretCache

	helmOp      fleetcontrollers.HelmOpController
	helmOpCache fleetcontrollers.HelmOpCache

	dynamicClient dynamicGetter
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &autoscalerHandler{
		clusterClient: clients.Provisioning.Cluster(),
		clusterCache:  clients.Provisioning.Cluster().Cache(),

		capiClusterCache:           clients.CAPI.Cluster().Cache(),
		capiMachineCache:           clients.CAPI.Machine().Cache(),
		capiMachineDeploymentCache: clients.CAPI.MachineDeployment().Cache(),

		globalRoleClient:        clients.Mgmt.GlobalRole(),
		globalRoleCache:         clients.Mgmt.GlobalRole().Cache(),
		globalRoleBindingClient: clients.Mgmt.GlobalRoleBinding(),
		globalRoleBindingCache:  clients.Mgmt.GlobalRoleBinding().Cache(),

		userClient: clients.Mgmt.User(),
		userCache:  clients.Mgmt.User().Cache(),

		tokenClient: clients.Mgmt.Token(),
		tokenCache:  clients.Mgmt.Token().Cache(),

		secretClient: clients.Core.Secret(),
		secretCache:  clients.Core.Secret().Cache(),

		helmOp:      clients.Fleet.HelmOp(),
		helmOpCache: clients.Fleet.HelmOp().Cache(),

		dynamicClient: clients.Dynamic,
	}

	// only run the "create" handlers if autoscaling is enabled. otherwise only run the cleanup handler
	// (in case the user disabled autoscaling after having it enabled
	if !features.ClusterAutoscaling.Enabled() {
		clients.CAPI.Cluster().OnChange(ctx, "autoscaler-cleanup", h.ensureCleanup)
		return
	}

	// warn the user if they have the autoscaling feature-flag enabled but no chart repo set,
	// then do not run the controller.
	if settings.ClusterAutoscalerChartRepository.Get() == "" {
		logrus.Warnf("[autoscaler] no value is set for the cluster-autoscaler-chart-repo Setting  - cannot enable autoscaling!")
		return
	}

	// Start background token renewal process, runs daily to refresh tokens that expire within a month.
	h.startTokenRenewal(ctx)

	clients.CAPI.Cluster().OnChange(ctx, "autoscaler-mgr", h.OnChange)
	clients.Fleet.HelmOp().OnChange(ctx, "autoscaler-status-sync", h.syncHelmOpStatus)

	s := machineDeploymentReplicaOverrider{
		clusterCache:     clients.Provisioning.Cluster().Cache(),
		clusterClient:    clients.Provisioning.Cluster(),
		capiClusterCache: clients.CAPI.Cluster().Cache(),
	}

	clients.CAPI.MachineDeployment().OnChange(ctx, "machinedeployment-replica-sync", s.syncMachinePoolReplicas)
}

// OnChange handles changes to CAPI clusters and manages autoscaler deployment.
// It checks if a cluster has autoscaling enabled and appropriate machine deployments,
// then sets up RBAC resources (user, role, role binding, token) and deploys the
// cluster-autoscaler chart. If autoscaling is paused, it scales down the autoscaler.
// If autoscaling is disabled, it cleans up all autoscaler resources.
// Returns the modified cluster or an error if any operation fails.
func (h *autoscalerHandler) OnChange(_ string, cluster *capi.Cluster) (*capi.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	// fetch appropriate capi resources related to this cluster (machineDeployments + machines)
	mds, err := h.capiMachineDeploymentCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machinedeployments for capi cluster %s/%s", cluster.Namespace, cluster.Name)
		return nil, err
	}

	machines, err := h.capiMachineCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machines for capi cluster %s/%s", cluster.Namespace, cluster.Name)
		return nil, err
	}

	// we have to check if autoscaling is paused first before removing resources
	//
	// if it isn't paused, then just check if autoscaling is enabled.
	// if cluster doesn't have autoscaling enabled, just ensure all resources are gone.
	//
	// if both of these pass, ensure the autoscaler is set up to function.
	if autoscalingPaused(cluster) {
		err := h.pauseAutoscaling(cluster)
		if err != nil {
			logrus.Debugf("[autoscaler] failed to pause autoscaling for %s/%s: %v", cluster.Namespace, cluster.Name, err)
			return nil, err
		}
		return cluster, nil
	} else if autoscalingEnabled := h.isAutoscalingEnabled(cluster, mds); !autoscalingEnabled {
		err := h.handleUninstall(cluster)
		if err != nil {
			logrus.Debugf("[autoscaler] failed to cleanup autoscaler resources for %s/%s: %v", cluster.Namespace, cluster.Name, err)
		}

		return cluster, nil
	}

	// Setup RBAC (user, role, role binding, token)
	kubeconfig, err := h.setupRBAC(cluster, mds, machines)
	if err != nil {
		return nil, err
	}

	// Deploy the cluster-autoscaler chart
	if err := h.deployChart(cluster, kubeconfig); err != nil {
		return nil, err
	}

	// enqueue the helmop in order to force a status-update on the v2prov cluster object (if it exists) from the helmOp
	h.helmOp.EnqueueAfter(cluster.Namespace, helmOpName(cluster), 5*time.Second)

	return cluster, nil
}

// isAutoscalingEnabled checks if autoscaling is enabled for the cluster and validates annotations.
// It verifies that the cluster has the autoscaler-enabled annotation set to true and that
// at least one machine deployment has both min-size and max-size annotations properly configured.
// Returns true if autoscaling should be enabled, false otherwise.
func (h *autoscalerHandler) isAutoscalingEnabled(cluster *capi.Cluster, mds []*capi.MachineDeployment) bool {
	if !capr.AutoscalerEnabledByCAPI(cluster, mds) {
		return false
	}

	logrus.Infof("[autoscaler] Cluster %s/%s is ready and has autoscaler enabled for at least one machine pool", cluster.Namespace, cluster.Name)
	return true
}

// autoscalingPaused returns true if the cluster autoscaler is paused at the cluster annotation level.
// It checks for the presence of the "provisioning.cattle.io/cluster-autoscaler-paused" annotation
// set to "true". This allows users to temporarily disable autoscaling without removing the configuration.
func autoscalingPaused(cluster *capi.Cluster) bool {
	return cluster.Annotations[capr.ClusterAutoscalerPausedAnnotation] == "true"
}

// pauseAutoscaling scales down the cluster-autoscaler to zero replicas if autoscaling is paused.
// It retrieves the existing kubeconfig secret and ensures the Fleet HelmOp for the cluster-autoscaler
// is scaled to 0 replicas, effectively pausing the autoscaler while maintaining all configuration.
// Returns an error if the secret retrieval or HelmOp scaling fails.
func (h *autoscalerHandler) pauseAutoscaling(cluster *capi.Cluster) error {
	// scale autoscaler down to zero if autoscaling is set to "pause"
	secret, err := h.secretCache.Get(cluster.Namespace, kubeconfigSecretName(cluster))
	if err != nil {
		return err
	}

	// scales cluster-autoscaler helm chart down to 0 replicas
	err = h.ensureFleetHelmOp(cluster, secret.ResourceVersion, 0)
	if err != nil {
		return err
	}

	return nil
}

// handleUninstall cleans up all autoscaler-related resources for the given cluster.
// It removes RBAC resources (user, role, role binding, token) and Fleet-managed
// resources (HelmOp) associated with the cluster. This is called when autoscaling
// is disabled or when the cluster is being deleted.
// Returns a combined error if any cleanup operations fail.
func (h *autoscalerHandler) handleUninstall(cluster *capi.Cluster) error {
	return errors.Join(h.cleanupRBAC(cluster), h.cleanupFleet(cluster))
}

// setupRBAC handles the complete RBAC setup for cluster autoscaling.
// It creates or retrieves the autoscaler user, ensures the global role with appropriate
// permissions exists, creates the global role binding between user and role, generates
// a token for the user, and creates the kubeconfig secret. Returns the kubeconfig
// secret that will be used to deploy the cluster-autoscaler chart.
func (h *autoscalerHandler) setupRBAC(capiCluster *capi.Cluster, mds []*capi.MachineDeployment, machines []*capi.Machine) (*v1.Secret, error) {
	logrus.Infof("[autoscaler] setting up rbac resources for cluster %s/%s", capiCluster.Namespace, capiCluster.Name)

	user, err := h.ensureUser(capiCluster)
	if err != nil {
		logrus.Errorf("[autoscaler] Failed to create user for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return nil, err
	}

	globalRole, err := h.ensureGlobalRole(capiCluster, mds, machines)
	if err != nil {
		logrus.Errorf("[autoscaler] Failed to create global role for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return nil, err
	}

	err = h.ensureGlobalRoleBinding(capiCluster, user.Username, globalRole.Name)
	if err != nil {
		logrus.Errorf("[autoscaler] Failed to create global role binding for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return nil, err
	}

	tokenStr, err := h.ensureUserToken(capiCluster, autoscalerUserName(capiCluster))
	if err != nil {
		logrus.Errorf("[autoscaler] Failed to create token for user %s: %v", user.Username, err)
		return nil, err
	}

	kubeconfig, err := h.ensureKubeconfigSecretUsingTemplate(capiCluster, tokenStr)
	if err != nil {
		logrus.Errorf("[autoscaler] Failed to create kubeconfig secret for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return nil, err
	}

	return kubeconfig, nil
}

// deployChart handles the deployment of the cluster-autoscaler chart to the target cluster.
// It creates a Fleet HelmOp with the provided kubeconfig secret reference, which will
// deploy the cluster-autoscaler chart to the downstream cluster. The HelmOp is created
// with 1 replica to enable autoscaling.
// Returns an error if the HelmOp creation fails.
func (h *autoscalerHandler) deployChart(capiCluster *capi.Cluster, kubeconfig *v1.Secret) error {
	logrus.Infof("[autoscaler] deploying cluster-autoscaler helm chart for cluster %s/%s", capiCluster.Namespace, capiCluster.Name)

	err := h.ensureFleetHelmOp(capiCluster, kubeconfig.ResourceVersion, 1)
	if err != nil {
		logrus.Errorf("[autoscaler] failed to create fleet-managed autoscaler helmop for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return err
	}

	return nil
}

// syncHelmOpStatus synchronizes the Helm operation status with the cluster's provisioning status.
// It monitors the HelmOp deployment progress and updates the corresponding provisioning cluster's
// status conditions based on the HelmOp state (ready, waiting, or error). This ensures that
// the cluster status accurately reflects the autoscaler deployment state.
// Returns the modified HelmOp or an error if status synchronization fails.
func (h *autoscalerHandler) syncHelmOpStatus(_ string, helmOp *fleet.HelmOp) (*fleet.HelmOp, error) {
	if helmOp == nil || helmOp.DeletionTimestamp != nil {
		return helmOp, nil
	}

	if helmOp.Labels[capi.ClusterNameLabel] == "" {
		return helmOp, nil
	}

	capiCluster, err := h.capiClusterCache.Get(helmOp.Namespace, helmOp.Labels[capi.ClusterNameLabel])
	if err != nil {
		return nil, err
	}

	cluster, err := capr.GetProvisioningClusterFromCAPICluster(capiCluster, h.clusterCache)
	// if there is not a provisioning cluster associated with this helmop its fine - just a non-v2prov cluster
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return helmOp, nil
	}

	// not populating the status condition on the cluster until the actual cluster is ready and the helm
	// operation will be progressing
	if !capr.Ready.IsTrue(cluster) {
		return helmOp, nil
	}

	cluster = cluster.DeepCopy()
	originalStatus := cluster.Status.DeepCopy()

	// looks like deployment completed successfully.
	if helmOp.Status.Summary.DesiredReady == helmOp.Status.Summary.Ready {
		capr.ClusterAutoscalerDeploymentReady.True(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "")
	} else if helmOp.Status.Summary.WaitApplied > 0 {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "[Waiting] autoscaler deployment pending")
	} else if helmOp.Status.Summary.ErrApplied > 0 {
		nonReadyResource := helmOp.Status.Summary.NonReadyResources[0].Message
		capr.ClusterAutoscalerDeploymentReady.Unknown(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "error encountered while deploying cluster-autoscaler: "+nonReadyResource)
	}

	// only update the v2prov cluster status if we changed it
	if !reflect.DeepEqual(originalStatus, cluster.Status) {
		_, err = h.clusterClient.UpdateStatus(cluster)
		if err != nil {
			logrus.Debugf("[autoscaler] failed to update provisioning cluster status: %v", err)
			h.helmOp.EnqueueAfter(helmOp.Namespace, helmOp.Name, 5*time.Second)
		}
	}

	return helmOp, nil
}

// ensureCleanup ensures all autoscaler resources are cleaned up when the autoscaling feature is disabled.
// This handler is registered only when ClusterAutoscaling.Enabled() returns false. It removes all
// RBAC and Fleet resources associated with the cluster to prevent resource leakage when autoscaling
// is disabled but clusters still exist.
func (h *autoscalerHandler) ensureCleanup(_ string, cluster *capi.Cluster) (*capi.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	err := h.handleUninstall(cluster)
	if err != nil {
		return nil, err
	}

	return cluster, nil
}
