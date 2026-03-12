package rkecontrolplanecondition

import (
	"context"
	"fmt"
	"sync"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	catalogv1 "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// systemUpgradeControllerConditionThrottleDuration defines how recently the SystemUpgradeControllerReady
// condition must have been updated before downstream API calls are skipped to reduce load.
const systemUpgradeControllerConditionThrottleDuration = 30 * time.Second

type handler struct {
	mgmtClusterName           string
	downstreamAppClient       catalogv1.AppClient
	rkeControlPlaneController rkecontrollers.RKEControlPlaneController
	pendingEnqueues           sync.Map
}

func Register(ctx context.Context, mgmtClusterName string, downstreamAppClient catalogv1.AppClient,
	rkeControlPlaneController rkecontrollers.RKEControlPlaneController) {

	h := handler{
		mgmtClusterName:           mgmtClusterName,
		downstreamAppClient:       downstreamAppClient,
		rkeControlPlaneController: rkeControlPlaneController,
	}

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, rkeControlPlaneController,
		"", "sync-system-upgrade-controller-condition", h.syncSystemUpgradeControllerCondition)
}

// syncSystemUpgradeControllerCondition checks the status of the system-upgrade-controller app in the target cluster
// and manages the SystemUpgradeControllerReady condition on the RKEControlPlane object
func (h *handler) syncSystemUpgradeControllerCondition(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if obj == nil {
		return status, nil
	}

	key := obj.Namespace + "/" + obj.Name
	if obj.DeletionTimestamp != nil {
		h.pendingEnqueues.Delete(key)
		return status, nil
	}

	if obj.Spec.ManagementClusterName != h.mgmtClusterName {
		return status, nil
	}

	targetVersion := settings.SystemUpgradeControllerChartVersion.Get()
	if targetVersion == "" {
		logrus.Warn("[rkecontrolplanecondition] the SystemUpgradeControllerChartVersion setting is not set")
		desiredReason := "the SystemUpgradeControllerChartVersion setting is not set"
		// Only update the condition if it is not already in the desired state, to avoid
		// unnecessary status updates and a tight reconcile loop while the setting is unset.
		if capr.SystemUpgradeControllerReady.IsFalse(&status) &&
			capr.SystemUpgradeControllerReady.GetReason(&status) == desiredReason &&
			capr.SystemUpgradeControllerReady.GetMessage(&status) == "" {
			return status, nil
		}
		capr.SystemUpgradeControllerReady.Reason(&status, desiredReason)
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		capr.SystemUpgradeControllerReady.LastUpdated(&status, time.Now().UTC().Format(time.RFC3339))
		return status, nil
	}

	// Skip if Rancher does not have a connection to the cluster
	if !status.AgentConnected {
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
			// In such cases, the condition should be reset
			capr.SystemUpgradeControllerReady.Reason(&status, "reset the condition")
			capr.SystemUpgradeControllerReady.Message(&status, "")
			capr.SystemUpgradeControllerReady.False(&status)
			capr.SystemUpgradeControllerReady.LastUpdated(&status, time.Now().UTC().Format(time.RFC3339))
			return status, nil
		}
	}

	// Skip if the condition was recently updated to avoid excessive downstream API calls
	if lastUpdated := capr.SystemUpgradeControllerReady.GetLastUpdated(&status); lastUpdated != "" {
		if lastUpdatedTime, parseErr := time.Parse(time.RFC3339, lastUpdated); parseErr == nil {
			if wait := systemUpgradeControllerConditionThrottleDuration - time.Since(lastUpdatedTime); wait > 0 {
				// Only call EnqueueAfter if there isn't already a pending enqueue for this object.
				// This avoids stacking up redundant delayed enqueues when the handler is invoked
				// multiple times during the throttle window.
				if _, alreadyPending := h.pendingEnqueues.LoadOrStore(key, struct{}{}); !alreadyPending {
					h.rkeControlPlaneController.EnqueueAfter(obj.Namespace, obj.Name, wait)
				}
				return status, nil
			}
		}
	}

	// Clear the pending enqueue flag since we're proceeding with a full reconciliation.
	h.pendingEnqueues.Delete(key)

	// In rare cases, downstream cluster may become disconnected but AgentConnected has not been updated to false.
	// If that happens, the following Get call can hang until it times out, causing this handler to take longer to return
	// and delaying the execution of other handlers.
	name := appName(obj.Spec.ClusterName)
	logrus.Debugf("[rkecontrolplanecondition] checking %s app in cluster %s", name, obj.Spec.ClusterName)
	app, err := h.downstreamAppClient.Get(namespace.System, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		capr.SystemUpgradeControllerReady.Reason(&status, err.Error())
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		capr.SystemUpgradeControllerReady.LastUpdated(&status, time.Now().UTC().Format(time.RFC3339))
		// don't return the error, otherwise the status won't be set to 'false'
		return status, nil
	} else if err != nil {
		return status, err
	}
	if app.DeletionTimestamp != nil {
		capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("waiting for %s to be reinstalled", app.Name))
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		capr.SystemUpgradeControllerReady.LastUpdated(&status, time.Now().UTC().Format(time.RFC3339))
		return status, nil
	}

	version := app.Spec.Chart.Metadata.Version
	if version != targetVersion {
		capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("waiting for %s to be updated to %s", app.Name, targetVersion))
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		capr.SystemUpgradeControllerReady.LastUpdated(&status, time.Now().UTC().Format(time.RFC3339))
		return status, nil
	}

	if app.Status.Summary.State != string(catalog.StatusDeployed) {
		capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("waiting for %s to be ready, current state %s", app.Name, app.Status.Summary.State))
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
		capr.SystemUpgradeControllerReady.LastUpdated(&status, time.Now().UTC().Format(time.RFC3339))
		return status, nil
	}

	capr.SystemUpgradeControllerReady.Reason(&status, "")
	capr.SystemUpgradeControllerReady.Message(&status, version)
	capr.SystemUpgradeControllerReady.True(&status)
	capr.SystemUpgradeControllerReady.LastUpdated(&status, time.Now().UTC().Format(time.RFC3339))

	return status, nil
}

func appName(clusterName string) string {
	return capr.SafeConcatName(capr.MaxHelmReleaseNameLength, "mcc",
		capr.SafeConcatName(48, clusterName, "managed", "system-upgrade-controller"))
}
