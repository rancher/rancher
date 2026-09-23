package operations

import (
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CancelForDeletion marks an operation deleted before its terminal handling completed as Canceled:
// the work it dispatched is no longer tracked by anything, so it can be reported neither as
// succeeded nor as failed. The terminal handler for the Canceled phase then runs as usual —
// honouring any canceled phase hook, releasing the beacon — before the finalizer is dropped.
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
// left — its outcome is asserted and will not change, so the phase it ended in stands and the
// request is declined. UpdateStatus reports the declined request on the Canceled condition, so
// setting the field is never silently ignored. The terminal check covers the already-Canceled case
// too, Canceled being terminal itself.
//
// This is deliberately narrower than CancelForDeletion, which acts on a terminal phase whose
// handling has not completed. The asymmetry is forced: a deleted operation has to release the beacon
// and retire its finalizer whatever phase it is in, or it would wait on a lifecycle hook that
// nothing will ever answer and never finish deleting. So the two verbs differ in scope — cancel
// stops the work, deletion removes the object and accepts what that implies — and deleting the
// operation is the remedy for a terminal phase hook whose delegate never returns the beacon.
//
// Callers reach this only for an operation which is not paused, so a paused operation is not
// canceled until it is resumed.
func CancelForRequest(spec *opv1alpha1.OperationSpec, status *opv1alpha1.OperationStatus) (opv1alpha1.OperationPhase, bool) {
	previous := status.Phase

	if !IsCanceled(spec) || IsTerminal(status.Phase) {
		return previous, false
	}

	status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")

	return previous, true
}

// Collectable reports whether a settled operation can be garbage collected: it reached a terminal
// phase, the controller finished handling that phase, its TTL has elapsed, and no lifecycle hook
// delegate is still expected to look at it.
//
// The last two conditions are what stop an operation waiting on a terminal phase hook from being
// deleted the moment its TTL passes, which would strand the delegate holding its beacon and bury
// the phase it actually finished in behind the cancellation that deletion performs.
func Collectable(op metav1.Object, spec *opv1alpha1.OperationSpec, status *opv1alpha1.OperationStatus) bool {
	return IsTerminal(status.Phase) &&
		IsTerminated(status) &&
		IsExpired(spec, status) &&
		!planv1alpha1.HasActiveLifecycleHook(op)
}
