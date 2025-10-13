package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"

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
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	// two pre-canned in order to stop execution during the autoscale main controller
	uninstalled = errors.New("autoscaler uninstalled")
	paused      = errors.New("autoscaler paused")
)

type autoscalerHandler struct {
	capiCluster v1beta1.ClusterCache

	clusterClient v2provcontrollers.ClusterClient
	clusterCache  v2provcontrollers.ClusterCache

	machineDeploymentCache v1beta1.MachineDeploymentCache
	machineCache           v1beta1.MachineCache

	globalRole             mgmtcontrollers.GlobalRoleClient
	globalRoleCache        mgmtcontrollers.GlobalRoleCache
	globalRoleBinding      mgmtcontrollers.GlobalRoleBindingClient
	globalRoleBindingCache mgmtcontrollers.GlobalRoleBindingCache

	user      mgmtcontrollers.UserClient
	userCache mgmtcontrollers.UserCache

	token      mgmtcontrollers.TokenClient
	tokenCache mgmtcontrollers.TokenCache

	secrets      wranglerv1.SecretController
	secretsCache wranglerv1.SecretCache

	helmOp      fleetcontrollers.HelmOpController
	helmOpCache fleetcontrollers.HelmOpCache
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	// only run these handlers if autoscaling is enabled
	if !features.ClusterAutoscaling.Enabled() {
		return
	}

	if settings.ClusterAutoscalerChartRepo.Get() == "" {
		logrus.Warnf("[autoscaler] no chart repo configured for autoscaler - cannot enable autoscaling!")
		return
	}

	h := &autoscalerHandler{
		clusterClient: clients.Provisioning.Cluster(),
		clusterCache:  clients.Provisioning.Cluster().Cache(),

		machineDeploymentCache: clients.CAPI.MachineDeployment().Cache(),
		machineCache:           clients.CAPI.Machine().Cache(),

		globalRole:             clients.Mgmt.GlobalRole(),
		globalRoleCache:        clients.Mgmt.GlobalRole().Cache(),
		globalRoleBinding:      clients.Mgmt.GlobalRoleBinding(),
		globalRoleBindingCache: clients.Mgmt.GlobalRoleBinding().Cache(),

		user:      clients.Mgmt.User(),
		userCache: clients.Mgmt.User().Cache(),

		token:      clients.Mgmt.Token(),
		tokenCache: clients.Mgmt.Token().Cache(),

		secrets:      clients.Core.Secret(),
		secretsCache: clients.Core.Secret().Cache(),

		helmOp:      clients.Fleet.HelmOp(),
		helmOpCache: clients.Fleet.HelmOp().Cache(),
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
	mds, err := h.machineDeploymentCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machinedeployments for capi cluster %v/%v", cluster.Namespace, cluster.Name)
		return cluster, err
	}

	machines, err := h.machineCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machines for capi cluster %v/%v", cluster.Namespace, cluster.Name)
		return cluster, err
	}

	// Handle cluster readiness check
	if autoscalingEnabled, err := h.isAutoscalingEnabled(cluster, mds); err != nil {
		return cluster, err
	} else if !autoscalingEnabled {
		err := h.handleUninstall(cluster)
		if err != nil {
			logrus.Debugf("[autoscaler] failed to cleanup autoscaler resources for %v/%v: %v", cluster.Namespace, cluster.Name, err)
		}

		return cluster, nil
	}

	err = h.handlePaused(cluster)
	if errors.Is(err, paused) {
		return cluster, nil
	} else if err != nil {
		return cluster, err
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

	return cluster, nil
}

// isAutoscalingEnabled checks if autoscaling is enabled for the cluster and validates annotations
func (h *autoscalerHandler) isAutoscalingEnabled(cluster *capi.Cluster, mds []*capi.MachineDeployment) (bool, error) {
	if !capr.AutoscalerEnabledByCAPI(cluster, mds) {
		logrus.Tracef("[autoscaler] No machine pools are configured for autoscaling - not setting up")
		return false, nil
	}

	if err := validateAutoscalerAnnotations(mds); err != nil {
		return false, err
	}

	logrus.Infof("[autoscaler] Cluster %s/%s is ready and has autoscaler enabled for at least one machine pool", cluster.Namespace, cluster.Name)
	return true, nil
}

// handlePaused checks to see if the cluster-autoscaler needs to be paused and does that if so
func (h *autoscalerHandler) handlePaused(cluster *capi.Cluster) error {
	// scale autoscaler down to zero if autoscaling is set to "pause"
	if cluster.Annotations[capr.ClusterAutoscalerEnabledAnnotation] == "paused" {
		secret, err := h.secretsCache.Get(cluster.Namespace, kubeconfigSecretName(cluster))
		if err != nil {
			return err
		}

		// scales cluster-autoscaler helm chart down to 0 replicas
		err = h.ensureFleetHelmOp(cluster, secret.ResourceVersion, 0)
		if err != nil {
			return err
		}

		return paused
	}

	return nil
}

// handleUninstall cleans up RBAC and Fleet resources for the given cluster.
func (h *autoscalerHandler) handleUninstall(cluster *capi.Cluster) error {
	return errors.Join(h.cleanupRbac(cluster), h.cleanupFleet(cluster))
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

	h.helmOp.Enqueue(capiCluster.Namespace, helmOpName(capiCluster))

	return nil
}

// these are validated by the webhook too - but doesn't hurt to double-validate.
func validateAutoscalerAnnotations(mds []*capi.MachineDeployment) error {
	for _, md := range mds {
		minSizeStr := md.Annotations[capi.AutoscalerMinSizeAnnotation]
		maxSizeStr := md.Annotations[capi.AutoscalerMaxSizeAnnotation]

		// not enabled for this machineDeployment
		if minSizeStr == "" && maxSizeStr == "" {
			return nil
		}

		minSize, _ := strconv.Atoi(minSizeStr)
		maxSize, _ := strconv.Atoi(maxSizeStr)

		if minSize > maxSize {
			return fmt.Errorf("cluster %s/%s pool %v has min size (%d) greater than max size (%d)", md.Namespace, md.Spec.ClusterName, md.Name, minSize, maxSize)
		}

		if !hasMinNodes(minSize, md.Spec.Template.Labels[capr.EtcdRoleLabel] != "", md.Spec.Template.Labels[capr.ControlPlaneRoleLabel] != "") {
			return fmt.Errorf("cluster %s/%s pool %v has controlplane/etcd role and has minimum of 0 nodes, 1 node is required", md.Namespace, md.Spec.ClusterName, md.Name)
		}
	}

	return nil
}

// hasMinNodes checks if minimum node requirements are met based on pool roles.
func hasMinNodes(minSize int, etcdRole, controlPlaneRole bool) bool {
	switch {
	case etcdRole:
		return minSize > 0
	case controlPlaneRole:
		return minSize > 0
	default:
		return true
	}
}

// Syncs the Helm operation status with the cluster's provisioning status based on deployment progress
func (h *autoscalerHandler) syncHelmOpStatus(_ string, helmOp *fleet.HelmOp) (*fleet.HelmOp, error) {
	if helmOp == nil || helmOp.DeletionTimestamp != nil {
		return helmOp, nil
	}

	if helmOp.Labels[capi.ClusterNameLabel] == "" {
		return helmOp, nil
	}

	cluster, err := h.clusterCache.Get(helmOp.Namespace, helmOp.Labels[capi.ClusterNameLabel])
	if err != nil {
		return helmOp, err
	}

	if !capr.Ready.IsTrue(cluster) {
		return helmOp, nil
	}

	cluster = cluster.DeepCopy()
	originalStatus := cluster.Status.DeepCopy()

	// looks like deployment completed successfully.
	if helmOp.Status.Summary.DesiredReady == helmOp.Status.Summary.Ready {
		capr.ClusterAutoscalerDeploymentReady.True(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "")
	}

	if helmOp.Status.Summary.WaitApplied > 0 {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "[Waiting] autoscaler deployment pending")
	}

	if helmOp.Status.Summary.ErrApplied > 0 {
		nonReadyResource := helmOp.Status.Summary.NonReadyResources[0].Message
		capr.ClusterAutoscalerDeploymentReady.Unknown(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "error encountered while deploying cluster-autoscaler: "+nonReadyResource)
	}

	// only update the v2prov cluster status if we changed it
	if !reflect.DeepEqual(originalStatus, cluster.Status) {
		_, err = h.clusterClient.UpdateStatus(cluster)
		if err != nil {
			logrus.Debugf("[autoscaler] failed to update provisioning cluster status: %v", err)
		}
	}

	return helmOp, nil
}
