package controllers

import (
	"errors"
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func phaseBasedReconcilerApplier(
	registrations registrationControllers.RegistrationController,
	regName string,
	reconciler types.HandlerReconcileErrorProcessor,
	regErr error,
	phase types.Phase,
) error {
	reconcileErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		curReg, getErr := registrations.Get(regName, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}

		prepareObj := curReg.DeepCopy()
		prepareObj = reconciler(prepareObj, regErr, phase)

		var reconcileUpdateErr error
		if prepareObj.Spec != curReg.Spec {
			_, reconcileUpdateErr = registrations.Update(prepareObj)
		}
		_, reconcileUpdateStatusErr := registrations.UpdateStatus(prepareObj)
		return errors.Join(reconcileUpdateErr, reconcileUpdateStatusErr)
	})

	err := fmt.Errorf("%s failed: %w", phase.GroupName(), regErr)
	if reconcileErr != nil {
		err = fmt.Errorf("%s failed with additional errors: %w, %w", phase.GroupName(), err, reconcileErr)
	}

	return err
}

func (h *handler) reconcileRegistration(registrationHandler SCCHandler, registrationObj *v1.Registration, regErr error, phase types.RegistrationPhase) error {
	phaseAdapter := func(reg *v1.Registration, err error, p types.Phase) *v1.Registration {
		specificPhase, _ := p.(types.RegistrationPhase)
		return registrationHandler.ReconcileRegisterError(reg, err, specificPhase)
	}
	return phaseBasedReconcilerApplier(h.registrations, registrationObj.Name, phaseAdapter, regErr, phase)
}

func (h *handler) reconcileActivation(registrationHandler SCCHandler, registrationObj *v1.Registration, regErr error, phase types.ActivationPhase) error {
	phaseAdapter := func(reg *v1.Registration, err error, p types.Phase) *v1.Registration {
		specificPhase, _ := p.(types.ActivationPhase)
		return registrationHandler.ReconcileActivateError(reg, err, specificPhase)
	}
	return phaseBasedReconcilerApplier(h.registrations, registrationObj.Name, phaseAdapter, regErr, phase)
}
