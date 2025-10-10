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
func (h *autoscalerHandler) OnChange(_ string, capiCluster *capi.Cluster) (*capi.Cluster, error) {
	if capiCluster == nil || capiCluster.DeletionTimestamp != nil {
		return capiCluster, nil
	}

	if settings.ClusterAutoscalerChartRepo.Get() == "" {
		logrus.Infof("[autoscaler] no chart repo configured for autoscaler - cannot configure")
		return capiCluster, nil
	}

	cluster, err := h.clusterCache.Get(capiCluster.Namespace, capiCluster.Name)
	if err != nil {
		logrus.Infof("failed to find v2prov cluster object for %v/%v", capiCluster.Namespace, capiCluster.Name)
		return capiCluster, nil
	}

	cluster = cluster.DeepCopy()

	// the only thing we're really syncing back to the rancher v2prov cluster object is the status - so just always do that on the way out
	capr.ClusterAutoscalerDeploymentReady.CreateUnknownIfNotExists(cluster)
	defer h.clusterClient.UpdateStatus(cluster)

	if !capr.Ready.IsTrue(cluster) {
		capr.ClusterAutoscalerDeploymentReady.Unknown(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "[Waiting] cluster is not ready")
		return capiCluster, nil
	}

	k8sminor := 0
	if cluster.Spec.KubernetesVersion != "" {
		version, err := semver.NewVersion(cluster.Spec.KubernetesVersion)
		if err != nil {
			return nil, err
		}
		k8sminor = int(version.Minor())
	} else {
		logrus.Infof("[autoscaler] no kubernetes version set for cluster %v/%v - latest version of cluster-autoscaler chart will be installed", cluster.Namespace, cluster.Name)
	}

	// scale autoscaler down to zero if autoscaling was "ready" and the annotation is present.
	if cluster.Annotations[capr.ClusterAutoscalerPausedAnnotation] != "" {
		// autoscaler is paused and we already marked it false, so just return.
		if capr.ClusterAutoscalerDeploymentReady.IsFalse(cluster) {
			return capiCluster, nil
		}

		secret, err := h.secretsCache.Get(cluster.Namespace, kubeconfigSecretName(capiCluster))
		if err != nil {
			return capiCluster, err
		}

		// scales cluster-autoscaler helm chart down to 0 replicas
		err = h.ensureFleetHelmOp(capiCluster, secret.ResourceVersion, k8sminor, 0)
		if err != nil {
			return capiCluster, err
		}

		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "autoscaling paused at cluster level")

		return capiCluster, nil
	}

	mds, err := h.machineDeploymentCache.List(capiCluster.Namespace, labels.SelectorFromSet(labels.Set{
		capi.ClusterNameLabel: capiCluster.Name,
	}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machinedeployments for capi cluster %v/%v", capiCluster.Namespace, capiCluster.Name)
		return capiCluster, nil
	}

	machines, err := h.machineCache.List(capiCluster.Namespace, labels.SelectorFromSet(labels.Set{
		capi.ClusterNameLabel: capiCluster.Name,
	}))
	if err != nil {
		logrus.Warnf("[autoscaler] failed to list machines for capi cluster %v/%v", capiCluster.Namespace, capiCluster.Name)
		return capiCluster, nil
	}

	// if the cluster has autoscaling disabled and the ClusterAutoscaling condition is True it must have been disabled, so lets uninstall the chart
	if !capr.AutoscalerEnabledByCAPI(capiCluster, mds) && capr.ClusterAutoscalerDeploymentReady.IsTrue(cluster) {
		err := h.uninstallHelmOp(capiCluster)
		if err != nil {
			logrus.Warnf("failed to delete helmop for cluster %v/%v", capiCluster.Namespace, capiCluster.Name)
			return nil, err
		}

		// remove the condition as well - like it was never there.
		cluster.Status.Conditions = slices.DeleteFunc(cluster.Status.Conditions, func(condition genericcondition.GenericCondition) bool {
			return condition.Type == string(capr.ClusterAutoscalerDeploymentReady)
		})

		return capiCluster, nil
	}

	if !capr.AutoscalerEnabledByCAPI(capiCluster, mds) {
		logrus.Tracef("[autoscaler] No machine pools are configured for autoscaling - not setting up")
		return capiCluster, nil
	}

	if err := validateAutoscalerAnnotations(mds); err != nil {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, err.Error())
		return capiCluster, err
	}

	logrus.Infof("[autoscaler] Cluster %s/%s is ready and has autoscaler enabled for at least one machine pool", capiCluster.Namespace, capiCluster.Name)

	user, err := h.ensureUser(capiCluster)
	if err != nil {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "failed to create autoscaler user")
		logrus.Errorf("[autoscaler] Failed to create user for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return capiCluster, err
	}

	globalRole, err := h.ensureGlobalRole(capiCluster, mds, machines)
	if err != nil {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "failed to create global role")
		logrus.Errorf("[autoscaler] Failed to create global role for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return capiCluster, err
	}

	err = h.ensureGlobalRoleBinding(capiCluster, user.Username, globalRole.Name)
	if err != nil {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "failed to create global role binding")
		logrus.Errorf("[autoscaler] Failed to create global role binding for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return capiCluster, err
	}

	tokenStr, err := h.ensureUserToken(capiCluster, autoscalerUserName(capiCluster))
	if err != nil {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "failed to create user token")
		logrus.Errorf("[autoscaler] Failed to create token for user %s: %v", user.Username, err)
		return capiCluster, err
	}

	kubeconfig, err := h.createKubeConfigSecretUsingTemplate(capiCluster, tokenStr)
	if err != nil {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "failed to create autoscaler kubeconfig")
		logrus.Errorf("[autoscaler] Failed to create kubeconfig secret for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return capiCluster, err
	}

	err = h.ensureFleetHelmOp(capiCluster, kubeconfig.ResourceVersion, k8sminor, 1)
	if err != nil {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "failed to create autoscaler HelmOp")
		logrus.Errorf("[autoscaler] failed to create fleet-managed autoscaler helmop for cluster %v/%v: %v", capiCluster.Namespace, capiCluster.Name, err)
		return capiCluster, err
	}

	h.helmOp.Enqueue(capiCluster.Namespace, helmOpName(capiCluster))
	return capiCluster, nil
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

	cluster, err := h.clusterCache.Get(helmOp.Namespace, helmOp.Labels[capi.ClusterNameLabel])
	if err != nil {
		return helmOp, err
	}

	cluster = cluster.DeepCopy()
	defer h.clusterClient.UpdateStatus(cluster)

	// looks like deployment completed successfully.
	if helmOp.Status.Summary.DesiredReady == helmOp.Status.Summary.Ready {
		capr.ClusterAutoscalerDeploymentReady.True(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "")
		return helmOp, nil
	}

	if helmOp.Status.Summary.WaitApplied > 0 {
		capr.ClusterAutoscalerDeploymentReady.False(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "[Waiting] autoscaler deployment pending")
		return helmOp, nil
	}

	if helmOp.Status.Summary.ErrApplied > 0 {
		nonReadyResource := helmOp.Status.Summary.NonReadyResources[0].Message
		capr.ClusterAutoscalerDeploymentReady.Unknown(cluster)
		capr.ClusterAutoscalerDeploymentReady.Message(cluster, "error encountered while deploying cluster-autoscaler: "+nonReadyResource)
		return helmOp, nil
	}

	return helmOp, nil
}
