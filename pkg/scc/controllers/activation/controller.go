package activation

import (
	"context"
	"errors"
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"time"
)

type Handler struct {
	ctx                context.Context
	registrations      registrationControllers.RegistrationController
	secrets            v1core.SecretController
	sccCredentials     *credentials.CredentialSecretsAdapter
	systemInfoExporter *systeminfo.InfoExporter
}

func New(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	sccCredentials *credentials.CredentialSecretsAdapter,
	systemInfoExporter *systeminfo.InfoExporter,
) *Handler {
	controller := &Handler{
		ctx:                ctx,
		registrations:      registrations,
		secrets:            secrets,
		sccCredentials:     sccCredentials,
		systemInfoExporter: systemInfoExporter,
	}

	return controller
}

func (h *Handler) Call(key string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, fmt.Errorf("received nil registration")
	}

	if v1.ResourceConditionFailure.IsTrue(registrationObj) ||
		v1.RegistrationConditionAnnounced.IsFalse(registrationObj) {
		return registrationObj, generic.ErrSkip
	}

	logrus.Infof("[scc.activations-controller]: Received registration ready for activations %q", key)
	logrus.Info("[scc.activations-controller]: registration ", registrationObj)

	var lastValidatedTS time.Time
	if registrationObj.Status.ActivationStatus.LastValidatedTS != nil {
		lastValidatedTS = registrationObj.Status.ActivationStatus.LastValidatedTS.Time
	}

	if *registrationObj.Spec.CheckNow && !lastValidatedTS.IsZero() {
		if registrationObj.Spec.Mode == v1.RegistrationModeOffline {
			updated := registrationObj.DeepCopy()
			// TODO(o&b): Also update the status to warn RegistrationModeOffline users that `CheckNow` does nothing
			// Better alternative, webhook prevent updates if mode=offline
			updated.Spec = *registrationObj.Spec.WithoutCheckNow()
			return h.registrations.Update(updated)
		} else {
			updated := registrationObj.DeepCopy()
			updated.Spec = *registrationObj.Spec.WithoutCheckNow()
			updated.Status.ActivationStatus.Activated = false
			updated.Status.ActivationStatus.LastValidatedTS = &metav1.Time{}
			updated.Status.ActivationStatus.ValidUntilTS = &metav1.Time{}
			v1.ResourceConditionProgressing.True(updated)
			v1.ResourceConditionReady.False(updated)
			v1.ResourceConditionDone.False(updated)

			var err error
			updated, err = h.registrations.UpdateStatus(updated)

			updated.Spec = *registrationObj.Spec.WithoutCheckNow()
			updated, err = h.registrations.Update(updated)
			return updated, err
		}
	}

	if !lastValidatedTS.IsZero() && time.Now().Sub(lastValidatedTS) < time.Hour {
		return registrationObj, nil
	}

	if registrationObj.Spec.Mode == v1.RegistrationModeOnline {
		registration, err := h.processOnlineActivation(registrationObj)
		if err != nil {
			return h.setReconcilingCondition(registration, err)
		}
	} else {
		registration, err := h.processOfflineActivation(registrationObj)
		if err != nil {
			return h.setReconcilingCondition(registration, err)
		}
	}

	return registrationObj, nil
}

func (h *Handler) setReconcilingCondition(registrationObj *v1.Registration, originalErr error) (*v1.Registration, error) {
	logrus.Info("[scc.registration-controller]: set reconciling condition")
	logrus.Error(originalErr)

	v1.ResourceConditionFailure.SetStatusBool(registrationObj, true)
	v1.ResourceConditionFailure.SetError(registrationObj, "", originalErr)

	registrationObj, err := h.registrations.UpdateStatus(registrationObj)
	if err != nil {
		return registrationObj, errors.New(originalErr.Error() + err.Error())
	}

	return registrationObj, originalErr
}

func (h *Handler) processOnlineActivation(registrationObj *v1.Registration) (*v1.Registration, error) {
	_ = h.sccCredentials.Refresh()
	regCode := suseconnect.FetchSccRegistrationCodeFrom(h.secrets, registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef)
	sccConnection := suseconnect.DefaultRancherConnection(h.sccCredentials.SccCredentials(), h.systemInfoExporter)

	identifier, version, arch := h.systemInfoExporter.Provider().GetProductIdentifier()
	metaData, product, err := sccConnection.Activate(identifier, version, arch, regCode)
	if err != nil {
		return registrationObj, err
	}
	logrus.Info(metaData)
	logrus.Info(product)

	// If no error, then system is still registered with valid activation status...
	keepAliveErr := sccConnection.KeepAlive()
	if keepAliveErr != nil {
		return registrationObj, keepAliveErr
	}

	logrus.Info("[scc.activation-controller]: Successfully registered activation")
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		registrationObj, err = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		updated := registrationObj.DeepCopy()
		v1.RegistrationConditionSccUrlReady.True(updated)
		v1.ResourceConditionProgressing.False(updated)
		v1.ResourceConditionReady.True(updated)
		v1.ResourceConditionDone.True(updated)
		updated.Status.ActivationStatus.LastValidatedTS = &metav1.Time{
			Time: time.Now(),
		}
		updated.Status.ActivationStatus.ValidUntilTS = &metav1.Time{
			Time: time.Now().Add(24 * time.Hour),
		}
		updated.Status.ActivationStatus.Activated = true
		registrationObj, err = h.registrations.UpdateStatus(updated)
		return err
	})

	return registrationObj, updateErr
}

func (h *Handler) processOfflineActivation(registrationObj *v1.Registration) (*v1.Registration, error) {
	return registrationObj, nil
}
