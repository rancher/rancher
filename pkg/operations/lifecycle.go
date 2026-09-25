package operations

import (
	"fmt"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/v3/pkg/condition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CancelForDeletion marks an operation deleted before its terminal handling completed as Canceled:
// the work it dispatched is no longer tracked by anything, so it can be reported neither as
// succeeded nor as failed. The terminal handler for the Canceled phase then runs as usual,
// honoring any canceled phase hook and releasing the beacon, before the finalizer is dropped.
//
// An operation which is already terminated keeps the phase it finished in; only the window before
// that counts as racing the operation, and in that window its beacon is still held, on its own
// behalf or a delegate's. An operation already in Canceled keeps the reason it was canceled for.
//
// Returns the phase the operation was in and whether it was canceled, so the caller can report the
// phase the deletion caught it in rather than the one it has just been moved to.
func CancelForDeletion(status *opv1alpha1.OperationStatus) (opv1alpha1.OperationPhase, bool) {
	previous := status.Phase

	if IsTerminated(status) || status.Phase == opv1alpha1.OperationPhaseCanceled {
		return previous, false
	}

	status.MarkCanceled(opv1alpha1.OperationDeletedReason, "operation deleted before terminal handling completed")

	return previous, true
}

// CancelForRequest marks an operation whose spec.Cancel is set as Canceled: the work was called off
// from outside, so it can be reported neither as succeeded nor as failed.
//
// Cancellation stops work in flight, and an operation which has reached a terminal phase has none
// left: its outcome is asserted and will not change, so the phase it ended in stands and the
// request is declined. UpdateStatus reports the declined request on the Canceled condition, so
// setting the field is never silently ignored. The terminal check covers the already-Canceled case
// too, Canceled being terminal itself.
//
// This is deliberately narrower than CancelForDeletion, which acts on a terminal phase whose
// handling has not completed. The asymmetry is forced: a deleted operation has to release the beacon
// and retire its finalizer whatever phase it is in, or it would wait on a lifecycle hook that
// nothing will ever answer and never finish deleting. So the two verbs differ in scope, cancel
// stops the work, deletion removes the object and accepts what that implies, and deleting the
// operation is the remedy for a terminal phase hook whose delegate never returns the beacon.
//
// Callers reach this only for an operation which is not paused, so a paused operation is not
// canceled until it is resumed.
//
// This records the decision; it does not carry it out. Stopping the work already dispatched is
// CancelDispatchedPlans, which the Canceled phase's terminal handler runs before it releases the
// beacon — a plan sitting in a machine-plan secret belongs to the agent, and moving the phase alone
// would leave it running while the next operation acquired the beacon.
func CancelForRequest(spec *opv1alpha1.OperationSpec, status *opv1alpha1.OperationStatus) (opv1alpha1.OperationPhase, bool) {
	previous := status.Phase

	if !IsCanceled(spec) || IsTerminal(status.Phase) {
		return previous, false
	}

	status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")

	return previous, true
}

// Collectable reports whether a settled operation can be garbage collected: it reached a terminal
// phase, the controller finished handling that phase, and its TTL has elapsed.
//
// Termination is the condition that carries the weight. It is recorded only once nothing is owed —
// the terminal phase hook was satisfied, or there was never anything to hand it, or it was
// abandoned because no beacon remained to delegate it on — so an operation still waiting on a hook
// is not collectable, and one whose hook can never be answered does not sit around forever. Testing
// the hook labels as well would reintroduce exactly that second case: a label nobody is coming back
// to clear would pin the operation for good.
func Collectable(spec *opv1alpha1.OperationSpec, status *opv1alpha1.OperationStatus) bool {
	return IsTerminal(status.Phase) &&
		IsTerminated(status) &&
		IsExpired(spec, status)
}

// UpdateStatus refreshes ObservedGeneration and every condition that is not the one the current
// phase handler owns. Every operation type reports its progress the same way, so they all share
// this; what is specific to an operation is the step it is on, and no condition here reports that.
//
// Division of labor for the outcome conditions (Succeeded / Failed / Aborted / Canceled): a phase
// handler records *why* the operation ended, by setting the reason and message on the condition
// matching the phase it moves to, the MarkSucceeded, MarkFailed, MarkAborted and MarkCanceled
// methods on OperationStatus do exactly that. This function asserts that outcome and denies the
// competing three, and owns the Finalized condition outright.
//
// The three states, in order:
//
//   - not terminal: the operation is still running. Progress conditions report where it is and
//     Finalized is False with NotFinalizedReason.
//   - terminal: the work is over and its outcome will not change, so the matching outcome condition
//     goes True (keeping the reason and message it was given at decision time) and the others go
//     False. The progress conditions are cleared.
//   - terminal and terminated: the controller is done with the operation too, so Finalized goes
//     True. Until then, it stays False with FinalizingReason, which is the only difference between
//     this state and the one above. "Done with" means nothing is owed: the terminal phase hook was
//     satisfied, or there was never anything to hand it, or the operation is being deleted and its
//     hooks are abandoned with it, or there is no longer a beacon to delegate them on and they were
//     abandoned for that — which Finalized reports with HookAbandonedReason.
func UpdateStatus(op metav1.Object, spec *opv1alpha1.OperationSpec, status *opv1alpha1.OperationStatus) {
	status.ObservedGeneration = op.GetGeneration()
	if spec.Paused {
		opv1alpha1.PausedCondition.True(status)
		opv1alpha1.PausedCondition.Reason(status, opv1alpha1.PausedReason)
		opv1alpha1.PausedCondition.Message(status, "Operation is paused")
	} else {
		opv1alpha1.PausedCondition.False(status)
		opv1alpha1.PausedCondition.Reason(status, opv1alpha1.NotPausedReason)
		opv1alpha1.PausedCondition.Message(status, "")
	}

	if !IsTerminal(status.Phase) {
		opv1alpha1.FinalizedCondition.False(status)
		opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.NotFinalizedReason)
		opv1alpha1.FinalizedCondition.Message(status, "")

		if status.Phase == opv1alpha1.OperationPhasePending {
			opv1alpha1.PendingCondition.True(status)
		} else if status.Phase == opv1alpha1.OperationPhaseInProgress {
			opv1alpha1.PendingCondition.False(status)
			opv1alpha1.PendingCondition.Reason(status, opv1alpha1.InProgressReason)
			opv1alpha1.PendingCondition.Message(status, "Operation now in progress")
		}

		return
	}

	outcome, summary := opv1alpha1.OutcomeConditionFor(status.Phase)

	// The outcome is asserted as soon as the terminal phase is reached: the work is over and the
	// result will not change. The reason and message the phase handler recorded are left in place:
	// they are the record of why the operation ended.
	outcome.True(status)

	for cond, reason := range map[condition.Cond]string{
		opv1alpha1.SucceededCondition: opv1alpha1.NotSuccessfulReason,
		opv1alpha1.FailedCondition:    opv1alpha1.NotFailedReason,
		opv1alpha1.AbortedCondition:   opv1alpha1.NotAbortedReason,
		opv1alpha1.CanceledCondition:  opv1alpha1.NotCanceledReason,
	} {
		if cond == outcome {
			continue
		}
		cond.False(status)
		cond.Reason(status, reason)
		cond.Message(status, summary)
	}

	// A cancellation requested after the operation reached a terminal phase changes nothing: there
	// is no work left to call off. Report it on the denied Canceled condition which is where an
	// observer looks to find out what became of the request, so that setting spec.Cancel is
	// acknowledged rather than silently passed over. See CancelForRequest for why it is declined,
	// and note that deleting the operation is what does act in this window.
	if outcome != opv1alpha1.CanceledCondition && IsCanceled(spec) {
		opv1alpha1.CanceledCondition.Reason(status, opv1alpha1.CancellationDeclinedReason)
		opv1alpha1.CanceledCondition.Message(status, fmt.Sprintf("cancellation requested, but the operation had already reached the %s phase", status.Phase))
	}

	// Terminated is the separate question of whether the controller is done with the operation, so
	// it is the only thing the terminal marker gates. An operation in this window is past being
	// cancellable, but is still canceled on its way out if it is deleted: see CancelForDeletion.
	terminated := IsTerminated(status)

	progressReason := opv1alpha1.FinalizingReason
	if terminated {
		progressReason = opv1alpha1.FinishedReason
	}

	opv1alpha1.PendingCondition.False(status)
	opv1alpha1.PendingCondition.Reason(status, progressReason)
	opv1alpha1.PendingCondition.Message(status, summary)
	opv1alpha1.InProgressCondition.False(status)
	opv1alpha1.InProgressCondition.Reason(status, progressReason)
	opv1alpha1.InProgressCondition.Message(status, summary)

	if !terminated {
		opv1alpha1.FinalizedCondition.False(status)

		// Read the delegate back off the operation rather than remembering it on a condition: the
		// hook label is the source of truth, so when the delegate clears it this reverts by itself.
		if delegate := TerminalHookDelegate(op, status.Phase); delegate != "" {
			opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.WaitingForDelegateReason)
			opv1alpha1.FinalizedCondition.Message(status, fmt.Sprintf("Waiting for delegates to finish: %v", delegate))
		} else {
			opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.FinalizingReason)
			opv1alpha1.FinalizedCondition.Message(status, "waiting for terminal handling to complete")
		}

		return
	}

	opv1alpha1.FinalizedCondition.True(status)

	// A hook label outliving termination is a hook that was abandoned rather than one that was
	// satisfied: the only way to get here with one still set is for the beacon it would have been
	// delegated on to have gone away first. Say so, rather than reporting the operation as having
	// finished cleanly and leaving the label looking like a hook that never fired.
	if HasActiveLifecycleHook(op) {
		opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.HookAbandonedReason)
		opv1alpha1.FinalizedCondition.Message(status, fmt.Sprintf("%s; lifecycle hooks were abandoned, no beacon remained to delegate them on", summary))

		return
	}

	opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.FinishedReason)
	opv1alpha1.FinalizedCondition.Message(status, summary)
}

// SetWaitingForDelegate reports, through the condition belonging to the phase currently being
// handled, that the operation's beacon has been handed to a lifecycle-hook delegate and the
// controller is waiting for it to finish. The phase itself does not move: the operation is still
// where it was, it just isn't the one driving the beacon.
func SetWaitingForDelegate(cond condition.Cond, status *opv1alpha1.OperationStatus, beacon *planv1alpha1.Beacon) {
	cond.True(status)
	cond.Reason(status, opv1alpha1.WaitingForDelegateReason)
	cond.Message(status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(beacon)))
}

// SetWaitingForPlan reports that the current step's plans have been handed to the system-agents and
// the controller is now waiting on their feedback.
func SetWaitingForPlan[S ~string](status *opv1alpha1.OperationStatus, step S, results []planapi.PlanStatus) {
	opv1alpha1.InProgressCondition.True(status)
	opv1alpha1.InProgressCondition.Reason(status, opv1alpha1.WaitingForPlanAppliedReason)
	opv1alpha1.InProgressCondition.Message(status, fmt.Sprintf("Waiting in step %s: %s", step, planapi.Message(results)))
}

// SetWaitingForSinglePlan is SetWaitingForPlan for the steps which walk nodes one at a time, where
// the plan's own message names the node it is waiting on and the step adds nothing.
func SetWaitingForSinglePlan(status *opv1alpha1.OperationStatus, planStatus *planapi.PlanStatus) {
	opv1alpha1.InProgressCondition.True(status)
	opv1alpha1.InProgressCondition.Reason(status, opv1alpha1.WaitingForPlanAppliedReason)
	opv1alpha1.InProgressCondition.Message(status, planapi.Message([]planapi.PlanStatus{*planStatus}))
}
