package operations

import (
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
)

func IsTerminal(phase opv1alpha1.OperationPhase) bool {
	return phase == opv1alpha1.OperationPhaseSucceeded ||
		phase == opv1alpha1.OperationPhaseFailed ||
		phase == opv1alpha1.OperationPhaseCanceled
}

func IsExpired(spec *opv1alpha1.OperationSpec, status *opv1alpha1.OperationStatus) bool {
	if spec.TTL < 0 {
		return false
	}

	start := status.LastUpdated.Time
	elapsed := time.Since(start)

	duration := time.Duration(spec.TTL) * time.Second
	return elapsed > duration
}
