package v1alpha1

import (
	"strings"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/v3/pkg/condition"
)

// The conditions below split into three kinds: progress conditions (Pending, InProgress, Paused)
// report what an operation is doing right now; outcome conditions (Succeeded, Failed, Aborted,
// Canceled) report how the work ended; and Finalized reports whether the controller is done with
// the operation altogether.
//
// An outcome condition goes True as soon as the operation reaches the matching terminal phase. At
// that point the work it was asked to do is over and its result will not change, but the
// controller is not necessarily finished: the terminal phase hook may still be delegated, the
// cluster may still be paused, and the beacon may still be held. Finalized covers that last stretch
// and goes True once it is complete (see OperationStatus.TerminatedAt).
//
// So the two questions an observer can ask are answered separately:
//
//   - "how did it turn out?" - the outcome conditions, available as early as possible;
//   - "is the controller done with it?" - Finalized, which is also the single target for
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

	// AbortedCondition represents the condition state for a task or process that called its own
	// work off, having found a condition it cannot proceed past.
	// True once the operation reaches the Aborted phase; see FinalizedCondition for whether the
	// controller has finished with it.
	AbortedCondition = condition.Cond("Aborted")

	// CanceledCondition represents the condition state for a task or process that has been canceled
	// from outside: by the user, by another controller, or by being deleted mid-flight.
	// True once the operation reaches the Canceled phase; see FinalizedCondition for whether the
	// controller has finished with it.
	CanceledCondition = condition.Cond("Canceled")

	// FinalizedCondition reports that the controller is done with the operation and nothing about it
	// will change again: it reached a terminal phase, its terminal phase hook has been satisfied,
	// any cluster it paused has been unpaused, and its beacon has been released. It is the summary
	// of the outcome conditions above, so an observer which does not care how the operation turned
	// out can wait on this one condition instead of racing several.
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

	// FinalizingReason surfaces when an operation has reached a terminal phase (its outcome is
	// asserted and will not change) but the controller has not finished with it: the terminal
	// phase hook may still be delegated, the cluster may still be paused, and the beacon may still
	// be held.
	FinalizingReason = "Finalizing"

	// HookAbandonedReason surfaces when an operation finished with a lifecycle hook label still on
	// it, because there was never going to be a beacon to hand that hook's delegate: the cluster or
	// the beacon went away first. The hook is not waited on, since nothing would ever satisfy it,
	// and the label is reported here instead so the abandonment is visible rather than looking like
	// a hook that simply never fired.
	HookAbandonedReason = "HookAbandoned"

	// NotFailedReason surfaces when an operation has not failed.
	NotFailedReason = "NotFailed"

	// NotSuccessfulReason surfaces when an operation has not completed successfully.
	NotSuccessfulReason = "NotSuccessful"

	// NotAbortedReason surfaces when an operation did not abort itself.
	NotAbortedReason = "NotAborted"

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

	// CancelRequestedReason surfaces when an operation was canceled because OperationSpec.Cancel
	// was set, by the user or by another controller that needed the operation to stop.
	CancelRequestedReason = "CancelRequested"

	// CancellationDeclinedReason surfaces on the Canceled condition when OperationSpec.Cancel was
	// set on an operation which had already reached a terminal phase. Cancellation stops work in
	// flight and there is none left, so the phase the operation ended in stands. It is reported
	// rather than passed over so that setting the field never goes unacknowledged.
	CancellationDeclinedReason = "CancellationDeclined"
)

// OutcomeConditionFor maps a terminal phase to the outcome condition that reports it, along with
// the one-line summary to use as the message on conditions that merely reflect the outcome rather
// than explaining it. The reason and message on the returned condition itself belong to whichever
// handler decided the outcome, and should not be overwritten with the summary.
//
// A non-terminal phase has no outcome, so it maps to FailedCondition: callers are expected to check
// the phase is terminal first, and treating an unrecognized phase as a failure matches how the
// operation controllers handle one.
func OutcomeConditionFor(phase OperationPhase) (condition.Cond, string) {
	switch phase {
	case OperationPhaseSucceeded:
		return SucceededCondition, "Operation completed successfully"
	case OperationPhaseAborted:
		return AbortedCondition, "Operation aborted"
	case OperationPhaseCanceled:
		return CanceledCondition, "Operation canceled"
	default:
		return FailedCondition, "Operation failed"
	}
}

func WaitingForDelegateMessage(beacon *planv1alpha1.Beacon) string {
	if beacon == nil {
		return ""
	}

	if len(beacon.Status.Delegates) == 0 {
		return ""
	}

	return strings.Join(beacon.Status.Delegates, ",")
}
