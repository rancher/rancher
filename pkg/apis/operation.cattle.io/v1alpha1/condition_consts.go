package v1alpha1

import (
	"strings"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/v3/pkg/condition"
)

// The conditions below split into three kinds: progress conditions (Pending, InProgress, Paused)
// report what an operation is doing right now; outcome conditions (Succeeded, Failed, Canceled)
// report how the work ended; and Finalized reports whether the controller is done with the
// operation altogether.
//
// An outcome condition goes True as soon as the operation reaches the matching terminal phase. At
// that point the work it was asked to do is over and its result will not change — but the
// controller is not necessarily finished: the terminal phase hook may still be delegated, the
// cluster may still be paused, and the beacon may still be held. Finalized covers that last stretch
// and goes True once it is complete (see OperationStatus.TerminatedAt).
//
// So the two questions an observer can ask are answered separately:
//
//   - "how did it turn out?" — the outcome conditions, available as early as possible;
//   - "is the controller done with it?" — Finalized, which is also the single target for
//     "it is over, whatever happened", since kubectl cannot wait on a disjunction of conditions.
//
// Waiting on an outcome plus Finalized together means "succeeded and fully wrapped up". Note that
// `kubectl wait` only ANDs repeated --for flags from v1.36 onwards; older clients silently honor
// just the last one.
var (
	// PendingCondition represents the condition state for a task or process that is awaiting execution or resolution.
	PendingCondition = condition.Cond("Pending")

	// InProgressCondition represents the condition state for a task or process that is currently in progress or being executed.
	InProgressCondition = condition.Cond("InProgress")

	// SucceededCondition represents the condition state for a task or process that completed successfully.
	// True once the operation reaches the Succeeded phase; see FinalizedCondition for whether the
	// controller has finished with it.
	SucceededCondition = condition.Cond("Succeeded")

	// FailedCondition represents the condition state for a task or process that has failed to complete successfully.
	// True once the operation reaches the Failed phase; see FinalizedCondition for whether the
	// controller has finished with it.
	FailedCondition = condition.Cond("Failed")

	// CanceledCondition represents the condition state for a task or process that has been canceled.
	// True once the operation reaches the Canceled phase; see FinalizedCondition for whether the
	// controller has finished with it.
	CanceledCondition = condition.Cond("Canceled")

	// FinalizedCondition reports that the controller is done with the operation and nothing about it
	// will change again: it reached a terminal phase, its terminal phase hook has been satisfied,
	// any cluster it paused has been unpaused, and its beacon has been released. It is the summary
	// of the three outcome conditions above, so an observer which does not care how the operation
	// turned out can wait on this one condition instead of racing two.
	FinalizedCondition = condition.Cond("Finalized")

	// PausedCondition represents the condition state for a task or process that has been paused.
	PausedCondition = condition.Cond("Paused")
)

const (
	// ClusterNotFoundReason surfaces when an operation fails because the cluster is not found.
	ClusterNotFoundReason = "ClusterNotFound"

	// BeaconLostReason surfaces when an operation fails because the beacon is lost.
	BeaconLostReason = "BeaconLost"

	// UnknownStepReason surfaces when an operation fails because the step is unknown.
	UnknownStepReason = "UnknownStep"

	// UnknownPhaseReason surfaces when an operation fails because the phase is unknown.
	UnknownPhaseReason = "UnknownPhase"

	// WaitingForRegistrationReason surfaces when an operation is waiting for registration.
	WaitingForRegistrationReason = "WaitingForRegistration"

	// WaitingForBeaconReason surfaces when an operation is waiting to acquire the beacon.
	WaitingForBeaconReason = "WaitingForBeacon"

	// WaitingForPlanAppliedReason surfaces when an operation is waiting for a node plan to be applied.
	WaitingForPlanAppliedReason = "WaitingForPlanApplied"

	WaitingForDelegateReason = "WaitingForDelegate"

	PlanFailedReason = "PlanFailed"

	// FinishedReason surfaces when an operation has reached a terminal state (success/failure).
	FinishedReason = "Finished"

	// NotFinalizedReason surfaces when an operation has not reached a terminal phase yet, and so
	// cannot have been finalized.
	NotFinalizedReason = "NotFinalized"

	// FinalizingReason surfaces when an operation has reached a terminal phase — its outcome is
	// asserted and will not change — but the controller has not finished with it: the terminal
	// phase hook may still be delegated, the cluster may still be paused, and the beacon may still
	// be held.
	FinalizingReason = "Finalizing"

	// NotFailedReason surfaces when an operation has not failed.
	NotFailedReason = "NotFailed"

	// NotSuccessfulReason surfaces when an operation has not completed successfully.
	NotSuccessfulReason = "NotSuccessful"

	// NotCanceledReason surfaces when an operation was not canceled.
	NotCanceledReason = "NotCanceled"

	// InProgressReason surfaces when an operation is currently in progress.
	InProgressReason = "InProgress"

	// PausedReason surfaces when an operation is paused.
	PausedReason = "Paused"

	// NotPausedReason surfaces when an operation is not paused.
	NotPausedReason = "NotPaused"

	// WaitingForSuitableLeaderReason surfaces when no suitable control-plane leader can be
	// elected for encryption key rotation yet. The operation will retry automatically.
	WaitingForSuitableLeaderReason = "WaitingForSuitableLeader"

	// WaitingForEncryptionKeyRotationReason surfaces when the rotate-keys plan has been applied
	// but the runtime secrets-encrypt status has not yet confirmed reencrypt_finished.
	WaitingForEncryptionKeyRotationReason = "WaitingForEncryptionKeyRotation"

	// PreflightCheckFailedReason surfaces when an operation with a preflight phase encounters an error.
	PreflightCheckFailedReason = "PreflightCheckFailed"

	// FailedReason surfaces when an operation is failed. It is a generic, non-descript reason.
	FailedReason = "Failed"

	// OperationDeletedReason surfaces when an operation was canceled because it was deleted before
	// terminal handling for it completed.
	OperationDeletedReason = "OperationDeleted"
)

func WaitingForDelegateMessage(beacon *planv1alpha1.Beacon) string {
	if beacon == nil {
		return ""
	}

	if len(beacon.Status.Delegates) == 0 {
		return ""
	}

	return strings.Join(beacon.Status.Delegates, ",")
}
