package autoscaler

import (
	"context"
	"errors"
	"reflect"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/lasso/pkg/dynamic"
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
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type autoscalerHandler struct {
	capiClusterCache           v1beta1.ClusterCache
	capiMachineCache           v1beta1.MachineCache
	capiMachineDeploymentCache v1beta1.MachineDeploymentCache

	clusterClient v2provcontrollers.ClusterClient
	clusterCache  v2provcontrollers.ClusterCache

	globalRole      mgmtcontrollers.GlobalRoleClient
	globalRoleCache mgmtcontrollers.GlobalRoleCache

	globalRoleBinding      mgmtcontrollers.GlobalRoleBindingClient
	globalRoleBindingCache mgmtcontrollers.GlobalRoleBindingCache

	user      mgmtcontrollers.UserClient
	userCache mgmtcontrollers.UserCache

	token      mgmtcontrollers.TokenClient
	tokenCache mgmtcontrollers.TokenCache

	secret      wranglerv1.SecretController
	secretCache wranglerv1.SecretCache

	helmOp      fleetcontrollers.HelmOpController
	helmOpCache fleetcontrollers.HelmOpCache

	dynamicClient *dynamic.Controller
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	// only run these handlers if autoscaling is enabled
	if !features.ClusterAutoscaling.Enabled() {
		return
	}

	// warn the user if they have the autoscaling feature-flag enabled but no chart repo set,
	// then do not run the controller.
	if settings.ClusterAutoscalerChartRepo.Get() == "" {
		logrus.Warnf("[autoscaler] no chart repo configured for autoscaler - cannot enable autoscaling!")
		return
	}

	h := &autoscalerHandler{
		clusterClient: clients.Provisioning.Cluster(),
		clusterCache:  clients.Provisioning.Cluster().Cache(),

		capiClusterCache:           clients.CAPI.Cluster().Cache(),
		capiMachineCache:           clients.CAPI.Machine().Cache(),
		capiMachineDeploymentCache: clients.CAPI.MachineDeployment().Cache(),

		globalRole:             clients.Mgmt.GlobalRole(),
		globalRoleCache:        clients.Mgmt.GlobalRole().Cache(),
		globalRoleBinding:      clients.Mgmt.GlobalRoleBinding(),
		globalRoleBindingCache: clients.Mgmt.GlobalRoleBinding().Cache(),

		user:      clients.Mgmt.User(),
		userCache: clients.Mgmt.User().Cache(),

		token:      clients.Mgmt.Token(),
		tokenCache: clients.Mgmt.Token().Cache(),

		secret:      clients.Core.Secret(),
		secretCache: clients.Core.Secret().Cache(),

		helmOp:      clients.Fleet.HelmOp(),
		helmOpCache: clients.Fleet.HelmOp().Cache(),

		dynamicClient: clients.Dynamic,
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

// OnChange checks if a capi cluster has the autoscaler-enabled annotation as well as any machinedeployments which
// are set up for autoscaling. if so it does the dance of setting up a rancher user with appropriate rbac and
// deploys the cluster-autoscaler chart to the downstream cluster once the cluster is ready.
func (h *autoscalerHandler) OnChange(_ string, cluster *capi.Cluster) (*capi.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	// fetch appropriate capi resources related to this cluster (machineDeployments + machines)
	mds, err := h.capiMachineDeploymentCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machinedeployments for capi cluster %v/%v", cluster.Namespace, cluster.Name)
		return cluster, err
	}

	machines, err := h.capiMachineCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machines for capi cluster %v/%v", cluster.Namespace, cluster.Name)
		return cluster, err
	}

	// we have to check if autoscaling is paused first before removing resources
	//
	// if it isn't paused, then just check if autoscaling is enabled.
	// if cluster doesn't have autoscaling enabled, just ensure all resources are gone.
	//
	// if both of these pass, ensure the autoscaler is set up to function.
	if autoscalingPaused(cluster) {
		return cluster, h.pauseAutoscaling(cluster)
	} else if autoscalingEnabled := h.isAutoscalingEnabled(cluster, mds); !autoscalingEnabled {
		err := h.handleUninstall(cluster)
		if err != nil {
			logrus.Debugf("[autoscaler] failed to cleanup autoscaler resources for %v/%v: %v", cluster.Namespace, cluster.Name, err)
		}

		return cluster, nil
	}

	// Setup RBAC (user, role, role binding, token)
	kubeconfig, err := h.setupRBAC(cluster, mds, machines)
	if err != nil {
		return cluster, err
	}

	// Deploy the cluster-autoscaler chart
	if err := h.deployChart(cluster, kubeconfig); err != nil {
		return cluster, err
	}

	// enqueue the helmop in order to force a status-update on the v2prov cluster object (if it exists) from the helmOp
	h.helmOp.Enqueue(cluster.Namespace, helmOpName(cluster))

	return cluster, nil
}

// isAutoscalingEnabled checks if autoscaling is enabled for the cluster and validates annotations
func (h *autoscalerHandler) isAutoscalingEnabled(cluster *capi.Cluster, mds []*capi.MachineDeployment) bool {
	if !capr.AutoscalerEnabledByCAPI(cluster, mds) {
		return false
	}

	logrus.Infof("[autoscaler] Cluster %s/%s is ready and has autoscaler enabled for at least one machine pool", cluster.Namespace, cluster.Name)
	return true
}

// Returns true if the cluster autoscaler is paused at the cluster annotation level
func autoscalingPaused(cluster *capi.Cluster) bool {
	return cluster.Annotations[capr.ClusterAutoscalerEnabledAnnotation] == "paused"
}

// pauseAutoscaling checks to see if the cluster-autoscaler needs to be paused and does that if so
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

// handleUninstall cleans up RBAC and Fleet resources for the given cluster.
func (h *autoscalerHandler) handleUninstall(cluster *capi.Cluster) error {
	return errors.Join(h.cleanupRBAC(cluster), h.cleanupFleet(cluster))
}

// setupRBAC handles user/token creation and RBAC setup
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

	kubeconfig, err := h.createKubeConfigSecretUsingTemplate(capiCluster, tokenStr)
	if err != nil {
		logrus.Errorf("[autoscaler] Failed to create kubeconfig secret for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return nil, err
	}

	return kubeconfig, nil
}

// deployChart handles the deployment of the cluster-autoscaler chart
func (h *autoscalerHandler) deployChart(capiCluster *capi.Cluster, kubeconfig *v1.Secret) error {
	logrus.Infof("[autoscaler] deploying cluster-autoscaler helm chart for cluster %s/%s", capiCluster.Namespace, capiCluster.Name)

	err := h.ensureFleetHelmOp(capiCluster, kubeconfig.ResourceVersion, 1)
	if err != nil {
		logrus.Errorf("[autoscaler] failed to create fleet-managed autoscaler helmop for cluster %v/%v: %v", capiCluster.Namespace, capiCluster.Name, err)
		return err
	}

	return nil
}

// Syncs the Helm operation status with the cluster's provisioning status based on deployment progress
func (h *autoscalerHandler) syncHelmOpStatus(_ string, helmOp *fleet.HelmOp) (*fleet.HelmOp, error) {
	if helmOp == nil || helmOp.DeletionTimestamp != nil {
		return helmOp, nil
	}

	if helmOp.Labels[capi.ClusterNameLabel] == "" {
		return helmOp, nil
	}

	capiCluster, err := h.capiClusterCache.Get(helmOp.Namespace, helmOp.Labels[capi.ClusterNameLabel])
	if err != nil {
		return helmOp, err
	}

	cluster, err := capr.GetProvisioningClusterFromCAPICluster(capiCluster, h.clusterCache)
	// if there is not a provisioning cluster associated with this helmop its fine - just a non-v2prov cluster
	if err != nil && cluster == nil {
		return helmOp, nil
	} else if err != nil {
		return helmOp, err
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
			h.helmOp.Enqueue(helmOp.Namespace, helmOp.Name)
		}
	}

	return helmOp, nil
}
