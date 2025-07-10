package common

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/types"
)

// GetRegistrationReconcilers returns all shared reconcilers
func GetRegistrationReconcilers() []types.RegistrationFailureReconciler {
	return []types.RegistrationFailureReconciler{
		PrepareFailed,
	}
}

func PrepareFailed(regIn *v1.Registration, err error) *v1.Registration {
	v1.ResourceConditionProgressing.False(regIn)
	v1.ResourceConditionReady.False(regIn)

	v1.ResourceConditionFailure.SetError(regIn, "could not complete registration", err)

	return regIn
}
