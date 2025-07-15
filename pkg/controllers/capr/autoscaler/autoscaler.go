package autoscaler

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/Masterminds/semver/v3"
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
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type autoscalerHandler struct {
	capiCluster v1beta1.ClusterCache

	v2provClusterClient v2provcontrollers.ClusterClient
	v2provClusterCache  v2provcontrollers.ClusterCache

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
	h := &autoscalerHandler{
		v2provClusterClient: clients.Provisioning.Cluster(),
		v2provClusterCache:  clients.Provisioning.Cluster().Cache(),

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
		clusterCache:  clients.Provisioning.Cluster().Cache(),
		clusterClient: clients.Provisioning.Cluster(),
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

	if !features.Autoscaling.Enabled() {
		return cluster, nil
	}

	if settings.ClusterAutoscalerChartRepo.Get() == "" {
		logrus.Infof("[autoscaler] no chart repo configured for autoscaler - cannot configure")
		return cluster, nil
	}

	v2provCluster, err := h.v2provClusterCache.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		logrus.Infof("failed to find v2prov cluster object for %v/%v", cluster.Namespace, cluster.Name)
		return cluster, nil
	}

	// the only thing we're really syncing back to the rancher v2prov cluster object is the status - so just always do that on the way out
	capr.Autoscaler.CreateUnknownIfNotExists(v2provCluster)
	defer h.v2provClusterClient.UpdateStatus(v2provCluster)

	if !capr.Ready.IsTrue(v2provCluster) {
		capr.Autoscaler.Unknown(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, "[Waiting] cluster is not ready")
		return cluster, nil
	}

	version, err := semver.NewVersion(v2provCluster.Spec.KubernetesVersion)
	if err != nil {
		return nil, err
	}

	// scale autoscaler down to zero if autoscaling was "ready" and the annotation is present.
	if v2provCluster.Annotations[capr.AutoscalerPausedAnnotation] != "" {
		// autoscaler is paused and we already marked it false, so just return.
		if capr.Autoscaler.IsFalse(v2provCluster) {
			return cluster, nil
		}

		// scales cluster-autoscaler helm chart down to 0 replicas
		err := h.ensureFleetHelmOp(cluster, int(version.Minor()), 0)
		capr.Autoscaler.False(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, "autoscaling paused at cluster level")

		return cluster, err
	}

	mds, err := h.machineDeploymentCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{
		capi.ClusterNameLabel: cluster.Name,
	}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machinedeployments for capi cluster %v/%v", cluster.Namespace, cluster.Name)
		return cluster, nil
	}

	machines, err := h.machineCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{
		capi.ClusterNameLabel: cluster.Name,
	}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machines for capi cluster %v/%v", cluster.Namespace, cluster.Name)
		return cluster, nil
	}

	// if the cluster has autoscaling disabled and the Autoscaling condition is True it must have been disabled, so lets uninstall the chart
	if !capr.AutoscalerEnabledByCAPI(cluster, mds) && capr.Autoscaler.IsTrue(v2provCluster) {
		err := h.uninstallHelmOp(cluster)
		if err != nil {
			logrus.Warnf("failed to delete helmop for cluster %v/%v", cluster.Namespace, cluster.Name)
			return nil, err
		}

		// remove the condition as well - like it was never there.
		v2provCluster.Status.Conditions = slices.DeleteFunc(v2provCluster.Status.Conditions, func(condition genericcondition.GenericCondition) bool {
			return condition.Type == string(capr.Autoscaler)
		})

		return cluster, nil
	}

	if !capr.AutoscalerEnabledByCAPI(cluster, mds) {
		logrus.Tracef("[autoscaler] No machine pools are configured for autoscaling - not setting up")
		return cluster, nil
	}

	if err := validateAutoscalerAnnotations(mds); err != nil {
		capr.Autoscaler.False(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, err.Error())
		return cluster, err
	}

	logrus.Infof("[autoscaler] Cluster %s/%s is ready and has autoscaler enabled for at least one machine pool", cluster.Namespace, cluster.Name)

	user, err := h.ensureUser(cluster)
	if err != nil {
		capr.Autoscaler.False(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, "failed to create autoscaler user")
		logrus.Errorf("[autoscaler] Failed to create user for cluster %s/%s: %v", cluster.Namespace, cluster.Name, err)
		return cluster, err
	}

	globalRole, err := h.ensureGlobalRole(cluster, mds, machines)
	if err != nil {
		capr.Autoscaler.False(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, "failed to create global role")
		logrus.Errorf("[autoscaler] Failed to create global role for cluster %s/%s: %v", cluster.Namespace, cluster.Name, err)
		return cluster, err
	}

	err = h.ensureGlobalRoleBinding(cluster, user.Username, globalRole.Name)
	if err != nil {
		capr.Autoscaler.False(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, "failed to create global role binding")
		logrus.Errorf("[autoscaler] Failed to create global role binding for cluster %s/%s: %v", cluster.Namespace, cluster.Name, err)
		return cluster, err
	}

	token, err := h.ensureUserToken(cluster, autoscalerUserName(cluster))
	if err != nil {
		capr.Autoscaler.False(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, "failed to create user token")
		logrus.Errorf("[autoscaler] Failed to create token for user %s: %v", user.Username, err)
		return cluster, err
	}

	err = h.createKubeConfigSecretUsingTemplate(cluster, token)
	if err != nil {
		capr.Autoscaler.False(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, "failed to create autoscaler kubeconfig")
		logrus.Errorf("[autoscaler] Failed to create kubeconfig secret for cluster %s/%s: %v", cluster.Namespace, cluster.Name, err)
		return cluster, err
	}

	err = h.ensureFleetHelmOp(cluster, int(version.Minor()), 1)
	if err != nil {
		capr.Autoscaler.False(v2provCluster)
		capr.Autoscaler.Message(v2provCluster, "failed to create autoscaler HelmOp")
		logrus.Errorf("[autoscaler] failed to create fleet-managed autoscaler helmop for cluster %v/%v: %v", cluster.Namespace, cluster.Name, err)
		return cluster, err
	}

	h.helmOp.Enqueue(cluster.Namespace, helmOpName(cluster))
	return cluster, nil
}

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

func (h *autoscalerHandler) syncHelmOpStatus(_ string, helmOp *fleet.HelmOp) (*fleet.HelmOp, error) {
	if helmOp == nil || helmOp.DeletionTimestamp != nil {
		return helmOp, nil
	}

	if helmOp.Labels[capi.ClusterNameLabel] == "" {
		return helmOp, nil
	}

	cluster, err := h.v2provClusterCache.Get(helmOp.Namespace, helmOp.Labels[capi.ClusterNameLabel])
	if err != nil {
		return helmOp, err
	}

	defer h.v2provClusterClient.UpdateStatus(cluster)

	// looks like deployment completed successfully.
	if helmOp.Status.Summary.DesiredReady == helmOp.Status.Summary.Ready {
		capr.Autoscaler.True(cluster)
		capr.Autoscaler.Message(cluster, "")
		return helmOp, nil
	}

	if helmOp.Status.Summary.WaitApplied > 0 {
		capr.Autoscaler.False(cluster)
		capr.Autoscaler.Message(cluster, "[Waiting] autoscaler deployment pending")
		return helmOp, nil
	}

	if helmOp.Status.Summary.ErrApplied > 0 {
		nonReadyResource := helmOp.Status.Summary.NonReadyResources[0].Message
		capr.Autoscaler.Unknown(cluster)
		capr.Autoscaler.Message(cluster, "error encountered while deploying cluster-autoscaler: "+nonReadyResource)
		return helmOp, nil
	}

	return helmOp, nil
}
