package rkecontrolplanecondition

import (
	"context"
	"fmt"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/managesystemagent"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	catalogv1 "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	upgradev1 "github.com/rancher/rancher/pkg/generated/controllers/upgrade.cattle.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type handler struct {
	mgmtClusterName      string
	clusterCache         provisioningcontrollers.ClusterCache
	downstreamAppClient  catalogv1.AppClient
	downstreamPlanClient upgradev1.PlanClient
}

func Register(ctx context.Context, context *config.UserContext) {
	mgmtWrangler := context.Management.Wrangler

	h := handler{
		mgmtClusterName:      context.ClusterName,
		clusterCache:         context.Management.Wrangler.Provisioning.Cluster().Cache(),
		downstreamAppClient:  context.Catalog.V1().App(),
		downstreamPlanClient: context.Plan.V1().Plan(),
	}

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, mgmtWrangler.RKE.RKEControlPlane(),
		"", "sync-system-upgrade-controller-condition", h.syncSystemUpgradeControllerCondition)

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, mgmtWrangler.RKE.RKEControlPlane(),
		"", "sync-system-agent-upgrader-condition", h.syncSystemAgentUpgraderCondition)
}

// syncSystemUpgradeControllerCondition checks the status of the system-upgrade-controller app in the downstream cluster
// and sets a condition on the control-plane object
func (h *handler) syncSystemUpgradeControllerCondition(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return status, nil
	}
	if obj.Spec.ManagementClusterName != h.mgmtClusterName {
		return status, nil
	}

	cluster, err := h.getCluster()
	if err != nil {
		return status, err
	}
	// Skip if the cluster is undergoing an upgrade, provisioning, or not in the ready state
	if !(capr.Updated.IsTrue(cluster) && capr.Provisioned.IsTrue(cluster) && capr.Ready.IsTrue(cluster)) {
		logrus.Infof("[syncSystemUpgradeControllerCondition] cluster %s/%s: cluster is not ready, skip syncing condition", cluster.Namespace, cluster.Name)
		return status, nil
	}

	name := appName(obj.Spec.ClusterName)
	app, err := h.downstreamAppClient.Get(namespace.System, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		capr.SystemUpgradeControllerReady.Reason(&status, err.Error())
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		// don't return the error, otherwise the status won't be set to 'false'
		return status, nil
	} else if err != nil {
		return status, err
	}
	if app.DeletionTimestamp != nil {
		capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("waiting for %s to be reinstalled", app.Name))
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		return status, nil
	}

	targetVersion := settings.SystemUpgradeControllerChartVersion.Get()
	if targetVersion == "" {
		logrus.Warnf("[rkecontrolplanecondition] cluster %s/%s: SystemUpgradeControllerChartVersion setting is not set", cluster.Namespace, cluster.Name)
	}
	version := app.Spec.Chart.Metadata.Version
	if version != targetVersion && targetVersion != "" {
		capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("waiting for %s to be updated to %s", app.Name, targetVersion))
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		return status, nil
	}

	if app.Status.Summary.State != string(catalog.StatusDeployed) {
		capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("waiting for %s to be ready, current state %s", app.Name, app.Status.Summary.State))
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		return status, nil
	}

	capr.SystemUpgradeControllerReady.Reason(&status, "")
	capr.SystemUpgradeControllerReady.Message(&status, version)
	capr.SystemUpgradeControllerReady.True(&status)

	return status, nil
}

// syncSystemAgentUpgraderCondition checks the status of the system-agent-upgrader plan in the downstream cluster
// and sets a condition on the ControlPlane CR
func (h *handler) syncSystemAgentUpgraderCondition(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return status, nil
	}
	if obj.Spec.ManagementClusterName != h.mgmtClusterName {
		return status, nil
	}

	cluster, err := h.getCluster()
	if err != nil {
		return status, err
	}
	// Skip if the cluster is undergoing an upgrade, provisioning, or not in the ready state
	if !(capr.Updated.IsTrue(cluster) && capr.Provisioned.IsTrue(cluster) && capr.Ready.IsTrue(cluster)) {
		logrus.Infof("[syncSystemAgentUpgraderCondition] cluster %s/%s: cluster is not ready, skip syncing condition", cluster.Namespace, cluster.Name)
		return status, nil
	}

	plan, err := h.downstreamPlanClient.Get(namespace.System, managesystemagent.SystemAgentUpgrader, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// if we couldn't find the plan then we know it's not ready
		capr.SystemAgentUpgraded.Reason(&status, err.Error())
		capr.SystemAgentUpgraded.Message(&status, "")
		capr.SystemAgentUpgraded.False(&status)
		// don't return the error, otherwise the status won't be set to 'false'
		return status, nil
	} else if err != nil {
		return status, err
	}

	if plan.DeletionTimestamp != nil {
		capr.SystemAgentUpgraded.Reason(&status, "plan is deleted, waiting for new plan to be created")
		capr.SystemAgentUpgraded.Message(&status, "")
		capr.SystemAgentUpgraded.False(&status)
		return status, nil
	}

	// get the target version
	version := managesystemagent.SystemAgentUpgraderVersion()

	if version != plan.Status.LatestVersion || !planv1.PlanComplete.IsTrue(plan) {
		capr.SystemAgentUpgraded.Reason(&status, fmt.Sprintf("waiting for system-agent-upgrader Plan %s to complete execution", version))
		capr.SystemAgentUpgraded.Message(&status, "")
		capr.SystemAgentUpgraded.False(&status)
		return status, nil
	}

	capr.SystemAgentUpgraded.Reason(&status, "")
	capr.SystemAgentUpgraded.Message(&status, plan.Status.LatestVersion)
	capr.SystemAgentUpgraded.True(&status)

	return status, nil
}

func appName(clusterName string) string {
	return capr.SafeConcatName(capr.MaxHelmReleaseNameLength, "mcc",
		capr.SafeConcatName(48, clusterName, "managed", "system-upgrade-controller"))
}

// getCluster returns the provisioning cluster associated with the current userContext.
func (h *handler) getCluster() (*provv1.Cluster, error) {
	clusters, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.mgmtClusterName)
	if err != nil || len(clusters) != 1 {
		return nil, fmt.Errorf("error while retrieving cluster %s from cache via index: %w", h.mgmtClusterName, err)
	}
	return clusters[0], nil
}
