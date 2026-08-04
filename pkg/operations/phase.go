package operations

import (
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
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
