package usercontrollers

import (
	"slices"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/clustermanager"
)

// controllersFailedMessage is the message of the ControllersRunning condition when the controllers failed
// to start. It is the same whatever the failure: the error is logged by the replica that owns the cluster.
// Errors change from one attempt to the next even when the cause doesn't, and every change would be
// another update of the cluster.
const controllersFailedMessage = "Failed to start the cluster's controllers, retrying. See the logs of the Rancher replica that owns the cluster for details."

// syncStatus reports the state of the user controllers of cluster in its ControllersRunning and
// PreBootstrapped conditions. The controllers are started outside the controller loop, so the cluster
// manager only records how that went and enqueues the cluster; this handler is what writes it, on the
// object of the current sync. Only the cluster's owner reports, since it is the replica that runs the
// cluster's controllers.
//
// Nothing is reported while the controllers are starting: the conditions keep what the last start
// reported until this one is over.
func (u *userControllersController) syncStatus(cluster *v3.Cluster, status v3.ClusterStatus) (v3.ClusterStatus, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || !v3.ClusterConditionProvisioned.IsTrue(cluster) || !u.isOwner(cluster) {
		return status, nil
	}

	state := u.starter.ControllersState(cluster)
	if state.Phase == clustermanager.ControllersStopped {
		return status, nil
	}

	// cluster is the status handler's own copy, so it can be changed in place.
	cluster.Status = status
	before := slices.Clone(cluster.Status.Conditions)

	switch state.Phase {
	case clustermanager.ControllersRunning:
		// The controllers may still be running as a follower if this replica only just became the owner;
		// they are restarted as the owner by the sync, which reports once that is done.
		if state.Owner {
			v3.ClusterConditionControllersRunning.True(cluster)
			v3.ClusterConditionControllersRunning.Message(cluster, "")
		}
	case clustermanager.ControllersFailed:
		v3.ClusterConditionControllersRunning.False(cluster)
		v3.ClusterConditionControllersRunning.Message(cluster, controllersFailedMessage)
	}

	v3.ClusterConditionPreBootstrapped.CreateUnknownIfNotExists(cluster)
	if state.PreBootstrapped {
		v3.ClusterConditionPreBootstrapped.True(cluster)
	}

	if !slices.Equal(before, cluster.Status.Conditions) {
		util.SummarizeReadyCondition(cluster)
	}
	return cluster.Status, nil
}
