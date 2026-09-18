package v1alpha1

import (
	"strings"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/v3/pkg/condition"
)

// The conditions below split into progress conditions (Pending, InProgress, Paused), which report
// what an operation is doing right now, and outcome conditions (Succeeded, Failed, Canceled, and
// the Finalized summary), which report how it ended.
//
// An outcome condition is only True once the operation is terminated: its terminal phase hook has
// been satisfied and its beacon released, with nothing left for the controller to do. Reaching a
// terminal phase is not enough — see OperationStatus.TerminatedAt. In the window in between, the
// outcome is known but not yet asserted, so the matching outcome condition is Unknown while
// carrying the reason and message for the outcome, and the InProgress condition reports
// FinalizingReason.
//
// This makes each outcome condition a self-sufficient wait target — `Succeeded` means "succeeded
// and fully wrapped up", never "succeeded but the beacon is still held" — and makes Finalized the
// single target for "it is over, whatever happened", since kubectl cannot wait on a disjunction of
// conditions.
var (
	// PendingCondition represents the condition state for a task or process that is awaiting execution or resolution.
	PendingCondition = condition.Cond("Pending")

	// InProgressCondition represents the condition state for a task or process that is currently in progress or being executed.
	InProgressCondition = condition.Cond("InProgress")

	// SucceededCondition represents the condition state for a task or process that completed successfully.
	// True only once the operation has also terminated.
	SucceededCondition = condition.Cond("Succeeded")

	// FailedCondition represents the condition state for a task or process that has failed to complete successfully.
	// True only once the operation has also terminated.
	FailedCondition = condition.Cond("Failed")

	// CanceledCondition represents the condition state for a task or process that has been canceled.
	// True only once the operation has also terminated.
	CanceledCondition = condition.Cond("Canceled")

	// FinalizedCondition reports that the operation is over and will not change again, whatever the
	// outcome: it reached a terminal phase and its terminal handling completed. It is the summary
	// of the three outcome conditions above, and exists so that an observer which does not care
	// whether the operation succeeded can wait on one condition instead of racing two.
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

	// FinalizingReason surfaces when an operation's outcome is known — it has reached a terminal
	// phase — but its terminal handling has not completed: the terminal phase hook may still be
	// delegated, and the beacon has not been released. The outcome is reported by the phase and by
	// the reason on the matching outcome condition, but no outcome condition is True yet.
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
