package operations

import opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"

func IsPaused(spec *opv1alpha1.OperationSpec) bool {
	return spec.Paused
}
