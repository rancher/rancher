package operations

import opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"

// IsPaused returns true when the operation's Paused flag is set. Operation controllers must
// skip reconciliation of paused operations; the only allowed state change is the user clearing
// the flag. When true, the status handler typically reports the Paused condition but takes no
// other action.
func IsPaused(spec *opv1alpha1.OperationSpec) bool {
	return spec.Paused
}
