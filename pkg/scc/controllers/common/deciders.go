package common

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/types"
)

// GetRegistrationDeciders returns all shared deciders
func GetRegistrationDeciders() []types.RegistrationDecider {
	return []types.RegistrationDecider{
		RegistrationIsFailed,
		RegistrationNeedsSyncNow,
	}
}

func RegistrationIsFailed(regIn *v1.Registration) bool {
	return regIn.HasCondition(v1.ResourceConditionFailure) && v1.ResourceConditionFailure.IsTrue(regIn)
}

func RegistrationNeedsSyncNow(regIn *v1.Registration) bool {
	return regIn.Spec.SyncNow != nil && *regIn.Spec.SyncNow
}
