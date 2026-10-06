package operations

import (
	"fmt"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// SettleCluster leaves the cluster of an operation in a terminal phase paused and whitelisted, or
// not, as that phase requires. pastPointOfNoReturn reports whether the operation got past the step
// that pauses the cluster, judged by the operation from its phase and step: a terminal phase leaves
// both in place.
//
//   - Failed and Canceled, past the point of no return, make sure the cluster is paused and
//     whitelists restores. The rotations are still paused there, so for them this changes nothing; the
//     restore unpauses the cluster for its restart, so one stopped from then on pauses it again. The
//     operation reports this on Finalized with RestoreRequiredReason (see MarkRestoreRequired).
//     Before the point of no return there is nothing to undo, so they leave the cluster as it is.
//   - Rejected unpauses the cluster unless it carries a whitelist, whoever added it. Rejected is
//     reached before the operation pauses, so normally there is nothing to undo; a whitelisted
//     cluster was paused by an earlier operation and stays paused until a restore repairs it.
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
func SettleCluster(adapter Adapter, status *opv1alpha1.OperationStatus, pastPointOfNoReturn bool) error {
	if status.IsTerminated() {
		return nil
	}

	var err error
	switch status.Phase {
	case opv1alpha1.OperationPhaseFailed, opv1alpha1.OperationPhaseCanceled:
		if !pastPointOfNoReturn {
			return nil
		}
		if err = adapter.PauseCluster(true, WhitelistRestores); err == nil {
			MarkRestoreRequired(status)
		}
	case opv1alpha1.OperationPhaseRejected:
		err = adapter.PauseCluster(false, WhitelistKeepsPause)
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
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
