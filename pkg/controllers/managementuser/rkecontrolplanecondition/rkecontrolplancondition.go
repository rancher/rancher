package rkecontrolplanecondition

import (
	"context"
	"fmt"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/clusterconnected"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	catalogv1 "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	upgradev1 "github.com/rancher/rancher/pkg/generated/controllers/upgrade.cattle.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
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
	h := handler{
		mgmtClusterName:      context.ClusterName,
		clusterCache:         context.Management.Wrangler.Provisioning.Cluster().Cache(),
		downstreamAppClient:  context.Catalog.V1().App(),
		downstreamPlanClient: context.Plan.V1().Plan(),
	}

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, context.Management.Wrangler.RKE.RKEControlPlane(),
		"", "sync-system-upgrade-controller-condition", h.syncSystemUpgradeControllerCondition)
}

// syncSystemUpgradeControllerCondition checks the status of the system-upgrade-controller app in the downstream cluster
// and manages the SystemUpgradeControllerReady condition on the RKEControlPlane object
func (h *handler) syncSystemUpgradeControllerCondition(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return status, nil
	}
	if obj.Spec.ManagementClusterName != h.mgmtClusterName {
		return status, nil
	}

	targetVersion := settings.SystemUpgradeControllerChartVersion.Get()
	if targetVersion == "" {
		logrus.Warn("[rkecontrolplanecondition] the SystemUpgradeControllerChartVersion setting is not set")
		capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("the SystemUpgradeControllerChartVersion setting is not set"))
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		return status, nil
	}

	if capr.SystemUpgradeControllerReady.IsTrue(&status) {
		actual := capr.SystemUpgradeControllerReady.GetMessage(&status)
		if actual == targetVersion {
			// Skip if the target version of the app has been installed
			return status, nil
		} else if actual == "" {
			// If SystemUpgradeControllerReady is true but its message is empty, this may occur in scenarios where Rancher
			// is upgraded to 2.12.x, then rolled back to 2.11.x, and later re-upgraded to 2.12.x without restoring the local cluster.
			// In such cases, the condition should be rest
			capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("reset the condition"))
			capr.SystemUpgradeControllerReady.Message(&status, "")
			capr.SystemUpgradeControllerReady.False(&status)
			return status, nil
		}
	}

	cluster, err := h.getCluster()
	if err != nil {
		return status, err
	}
	// Skip if Rancher does not have a connection to the cluster
	if !clusterconnected.Connected.IsTrue(cluster) {
		return status, nil
	}

	// In rare cases, downstream cluster may become disconnected even if the Connected condition was recently true.
	// If that happens, the following Get call can hang until it times out, causing this handler to take longer to return
	// and delaying the execution of other handlers.
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

	version := app.Spec.Chart.Metadata.Version
	if version != targetVersion {
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
