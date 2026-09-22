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
//   - Termination closes the window. Cancellation applies for as long as the controller has not
//     finished handling the operation, which includes a terminal phase still waiting on its
//     lifecycle hook; once terminated there is nothing left to call off. See IsTerminated.
func IsCanceled(spec *opv1alpha1.OperationSpec) bool {
	return spec.Cancel
}
