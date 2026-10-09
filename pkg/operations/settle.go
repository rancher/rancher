package operations

import (
	"fmt"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// SettleCluster leaves the cluster of an operation in a terminal phase paused and whitelisted, or
// not, as that phase requires. requiresRestore reports whether the operation got past the step
// that pauses the cluster, judged by the operation from its phase and step: a terminal phase leaves
// both in place.
//
//   - Failed and Canceled, past the point of no return, make sure the cluster is paused and
//     whitelists restores. The rotations are still paused there, so for them this changes nothing; the
//     restore unpauses the cluster for its restart, so one stopped from then on pauses it again. The
//     operation reports this on Finalized with RestoreRequiredReason (see MarkRestoreRequired).
//     Before the point of no return there is nothing to undo, so they leave the cluster as it is.
//   - Rejected does nothing: an operation is only rejected before its point of no return, in its
//     preflight or for a conflicting operation while Pending, so it never paused the cluster. Whatever
//     pause the cluster has belongs to someone else (an earlier operation that left it requiring a
//     restore, the operation it conflicted with, or a user) and is theirs to lift.
//   - Succeeded does nothing here: an operation that paused the cluster unpaused it, and removed the
//     whitelist, in the reconcile that marked it succeeded.
//
// These writes are to the cluster object rather than a machine-plan, so they are made whether or
// not the operation still holds the beacon: paused and whitelisted is the safe state for a cluster an
// operation left part-way through. A cluster object that no longer exists has nothing to settle.
//
// It is done once, before the operation terminates. Terminal handling runs again on every reconcile
// until the operation is collected, and by then the cluster is no longer this operation's to change:
// an admin may have removed the whitelist by hand, or a later operation paused the cluster.
func SettleCluster(adapter Adapter, status *opv1alpha1.OperationStatus, requiresRestore bool) error {
	if status.IsTerminated() {
		return nil
	}

	switch status.Phase {
	case opv1alpha1.OperationPhaseFailed, opv1alpha1.OperationPhaseCanceled:
	default:
		return nil
	}
	if !requiresRestore {
		return nil
	}

	err := adapter.PauseCluster(true, WhitelistRestores)
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	MarkRestoreRequired(status)
	return nil
}

// MarkRestoreRequired records on Finalized that the operation left its cluster paused and
// whitelisted, so that only an etcd snapshot restore may run on it until one has succeeded. The
// outcome condition is left alone, since its reason and message are the record of why the operation
// ended. UpdateStatus keeps the note once the operation has terminated.
func MarkRestoreRequired(status *opv1alpha1.OperationStatus) {
	_, summary := opv1alpha1.OutcomeConditionFor(status.Phase)
	opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.RestoreRequiredReason)
	opv1alpha1.FinalizedCondition.Message(status, fmt.Sprintf("%s; %s", summary, restoreRequiredNote))
}

// restoreRequiredNote is the part of the RestoreRequiredReason message that says what the operation
// left behind.
const restoreRequiredNote = "the cluster was left paused part-way through, and only an etcd snapshot restore may run on it until one succeeds"
