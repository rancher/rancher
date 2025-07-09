package controllers

import (
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func makeRegistrationReconcileRetry(
	registrations registrationControllers.RegistrationController,
	regName string,
	reconciler types.HandlerReconcileErrorProcessor,
	regErr error,
	phase types.Phase,
) types.RegistrationReconcileRetry {
	return func() error {
		curReg, getErr := registrations.Get(regName, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}

		prepareObj := curReg.DeepCopy()
		prepareObj = reconciler(prepareObj, regErr, phase)

		_, reconcileUpdateErr := registrations.Update(prepareObj)
		return reconcileUpdateErr
	}
}

func genericReconcilerApplier(
	registrations registrationControllers.RegistrationController,
	regName string,
	reconciler types.HandlerReconcileErrorProcessor,
	regErr error,
	phase types.Phase,
) error {
	reconcileErr := retry.RetryOnConflict(retry.DefaultRetry, makeRegistrationReconcileRetry(registrations, regName, reconciler, regErr, phase))

	err := fmt.Errorf("%s failed: %w", phase.GroupName(), regErr)
	if reconcileErr != nil {
		err = fmt.Errorf("%s failed with additional errors: %w, %w", phase.GroupName(), err, reconcileErr)
	}

	return err
}

func (h *handler) reconcileRegistration(registrationHandler SCCHandler, registrationObj *v1.Registration, regErr error, phase types.RegistrationPhase) error {
	adapter := func(reg *v1.Registration, err error, p types.Phase) *v1.Registration {
		specificPhase, _ := p.(types.RegistrationPhase)
		return registrationHandler.ReconcileRegisterError(reg, err, specificPhase)
	}
	return genericReconcilerApplier(h.registrations, registrationObj.Name, adapter, regErr, phase)
}

func (h *handler) reconcileActivation(registrationHandler SCCHandler, registrationObj *v1.Registration, regErr error, phase types.ActivationPhase) error {
	adapter := func(reg *v1.Registration, err error, p types.Phase) *v1.Registration {
		specificPhase, _ := p.(types.ActivationPhase)
		return registrationHandler.ReconcileActivateError(reg, err, specificPhase)
	}
	return genericReconcilerApplier(h.registrations, registrationObj.Name, adapter, regErr, phase)
}
