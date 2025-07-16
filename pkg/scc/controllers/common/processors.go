package common

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRegistrationProcessors returns all shared processors
func GetRegistrationProcessors() []types.RegistrationProcessor {
	return []types.RegistrationProcessor{}
}

// GetRegistrationStatusProcessors returns all shared processors
func GetRegistrationStatusProcessors() []types.RegistrationStatusProcessor {
	return []types.RegistrationStatusProcessor{
		PrepareSuccessfulActivation,
	}
}

func PrepareSuccessfulActivation(regIn *v1.Registration) *v1.Registration {
	now := metav1.Now()
	v1.RegistrationConditionActivated.True(regIn)
	v1.ResourceConditionProgressing.False(regIn)
	v1.ResourceConditionReady.True(regIn)
	v1.ResourceConditionDone.True(regIn)
	regIn.Status.ActivationStatus.LastValidatedTS = &now
	regIn.Status.ActivationStatus.Activated = true

	return regIn
}
