package operations

import (
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
)

// IsTerminal returns true when the operation has reached a terminal phase: Succeeded, Failed, or
// Canceled. Terminal operations no longer dispatch plans or modify cluster state. The
// etcdsnapshotsave/etcdsnapshotrestore controllers use this to decide when to release the beacon
// and when to respect the TTL for automatic deletion.
func IsTerminal(phase opv1alpha1.OperationPhase) bool {
	return phase == opv1alpha1.OperationPhaseSucceeded ||
		phase == opv1alpha1.OperationPhaseFailed ||
		phase == opv1alpha1.OperationPhaseCanceled
}

// IsTerminated returns true when the controller has recorded that terminal handling for the
// operation completed — the terminal phase hook has been satisfied and the beacon has been
// released, so nothing is left for the operation's controller to do.
//
// This is strictly stronger than IsTerminal: an operation which has reached a terminal phase may
// still be waiting on a delegate to finish the terminal phase hook, in which case its beacon is
// still held on its behalf. Deleting an operation in that window cancels it.
func IsTerminated(status *opv1alpha1.OperationStatus) bool {
	return !status.TerminatedAt.IsZero()
}

// TerminalPhaseHookPrefix returns the lifecycle-hook label prefix whose delegate can defer the
// terminal handling of the given phase, or "" for a phase that has no terminal hook.
//
// Note there is deliberately no hook for the Finalized condition: hooks gate phases, and Finalized
// is a condition, not a phase. The hook that defers finalization is the one belonging to the
// terminal phase the operation ended in, which is what this returns.
func TerminalPhaseHookPrefix(phase opv1alpha1.OperationPhase) string {
	switch phase {
	case opv1alpha1.OperationPhaseSucceeded:
		return planv1alpha1.SucceededPhaseHookLabelPrefix
	case opv1alpha1.OperationPhaseFailed:
		return planv1alpha1.FailedPhaseHookLabelPrefix
	case opv1alpha1.OperationPhaseCanceled:
		return planv1alpha1.CanceledPhaseHookLabelPrefix
	}
	return ""
}

// IsExpired returns true when the operation has lived longer than its TTL measured from its
// status.LastUpdated timestamp. Expired terminal operations can be safely deleted because
// downstream controllers (system-agent, snapshotbackpopulate, etc.) have already seen the final
// state.
//
// A negative TTL disables expiration — the operation never expires. TTL=0 means "expire
// immediately" (useful for tests or one-shot operations where the caller immediately polls the
// result and doesn't need the CR to linger).
func IsExpired(spec *opv1alpha1.OperationSpec, status *opv1alpha1.OperationStatus) bool {
	if spec.TTL < 0 {
		return false
	}

	start := status.LastUpdated.Time
	elapsed := time.Since(start)

	duration := time.Duration(spec.TTL) * time.Second
	return elapsed > duration
}
