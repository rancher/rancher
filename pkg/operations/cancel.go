package operations

import opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"

// IsCanceled returns true when the operation's Cancel flag is set. Cancellation is requested from
// outside the operation — by the user, or by another controller that needs the operation to stop —
// and cannot be unset, so a controller observing it must drive the operation to the Canceled phase
// and dispatch no further work.
//
// Two things bound when that request is acted on, and both are deliberate:
//
//   - Paused is checked first. A paused operation halts reconciliation entirely, so its
//     cancellation is only observed once the pause is lifted. See IsPaused.
//   - A terminal phase closes the window. Cancellation stops work in flight, and an operation whose
//     outcome is already asserted has none left, so the request is declined and reported rather
//     than acted on. See IsTerminal. Deletion is the one thing that still stops an operation in
//     that state, because it has to; controllers must not rely on Cancel to reclaim a beacon from
//     an operation which has already finished its work.
func IsCanceled(spec *opv1alpha1.OperationSpec) bool {
	return spec.Cancel
}
