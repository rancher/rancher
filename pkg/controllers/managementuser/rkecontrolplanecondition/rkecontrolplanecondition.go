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

// systemUpgradeControllerConditionThrottleDuration defines the minimum interval between
// downstream API calls to check the system-upgrade-controller app status.
const systemUpgradeControllerConditionThrottleDuration = 30 * time.Second

// throttleState tracks throttle and enqueue deduplication state for a single RKEControlPlane object.
type throttleState struct {
	lastDownstreamCheck time.Time // when the downstream API was last called
	enqueuePending      bool      // whether an EnqueueAfter has already been scheduled for this throttle window
}

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
		capr.SystemUpgradeControllerReady.Reason(&status, "the SystemUpgradeControllerChartVersion setting is not set")
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.False(&status)
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
			return status, nil
		}
	}

	// Skip if a downstream API check was performed recently, to reduce load on downstream clusters.
	if val, ok := h.pendingEnqueues.Load(key); ok {
		ts := val.(*throttleState)
		if wait := systemUpgradeControllerConditionThrottleDuration - time.Since(ts.lastDownstreamCheck); wait > 0 {
			// Only call EnqueueAfter if there isn't already a pending enqueue for this object.
			// This avoids stacking up redundant delayed enqueues when the handler is invoked
			// multiple times during the throttle window.
			if !ts.enqueuePending {
				ts.enqueuePending = true
				h.rkeControlPlaneController.EnqueueAfter(obj.Namespace, obj.Name, wait)
			}
			return status, nil
		}
	}

	// Record that we are performing a downstream API check for throttle purposes.
	h.pendingEnqueues.Store(key, &throttleState{lastDownstreamCheck: time.Now()})

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
		// don't return the error, otherwise the status won't be set to 'false'
		return status, nil
	} else if err != nil {
		// Don't throttle retries on transient errors.
		h.pendingEnqueues.Delete(key)
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
