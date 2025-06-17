package shared

import "github.com/rancher/rancher/pkg/scc/types"

// GetRegistrationReconcilers returns all shared reconcilers
func GetRegistrationReconcilers() []types.RegistrationFailureReconciler {
	return []types.RegistrationFailureReconciler{}
}
