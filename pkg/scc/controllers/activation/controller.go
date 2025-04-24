package activation

import (
	"context"
	"errors"
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/util"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	"time"
)

type handler struct {
	ctx            context.Context
	registrations  registrationControllers.RegistrationController
	secrets        v1core.SecretController
	sccCredentials *credentials.CredentialSecretsAdapter
	systemInfo     *util.RancherSystemInfo
}

func Register(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	systemInfo *util.RancherSystemInfo,
) {
	controller := &handler{
		ctx:            ctx,
		registrations:  registrations,
		secrets:        secrets,
		sccCredentials: credentials.New(secrets),
		systemInfo:     systemInfo,
	}

	registrations.OnChange(ctx, "registrationActivations", controller.OnActivationChange)
	// TODO: EnqueueAfter - revalidate every 24 hours
	// Ex: https://github.com/rancher/rancher/blob/d6b40c3acd945f0c8fe463ff96d144561c9640c3/pkg/controllers/dashboard/helm/repo.go#L95
}

func (h *handler) OnActivationChange(key string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, fmt.Errorf("received nil registration")
	}

	// Ignore the registration until both of these are true
	if v1.ResourceConditionFailure.ToK8sCondition().IsTrue(registrationObj) ||
		!v1.ResourceConditionProgressing.ToK8sCondition().IsTrue(registrationObj) ||
		!v1.RegistrationConditionAnnounced.ToK8sCondition().IsTrue(registrationObj) ||
		!v1.RegistrationConditionSccUrlReady.ToK8sCondition().IsFalse(registrationObj) {
		return registrationObj, generic.ErrSkip
	}

	logrus.Infof("[scc.activations-controller]: Received registration ready for activations %q", key)
	logrus.Info("[scc.activations-controller]: registration ", registrationObj)

	var lastValidatedTS time.Time
	if registrationObj.Status.ActivationStatus.LastValidatedTS != "" {
		lastValidatedTS, _ = time.Parse(time.RFC3339, registrationObj.Status.ActivationStatus.LastValidatedTS)
	}

	if registrationObj.Spec.CheckNow && !lastValidatedTS.IsZero() {
		if registrationObj.Spec.Mode == v1.Offline {
			updated := registrationObj.DeepCopy()
			// TODO: Also update the status to warn Offline users that `CheckNow` does nothing
			// Better alternative, webhook prevent updates if mode=offline
			updated.Spec = *registrationObj.Spec.WithoutCheckNow()
			return h.registrations.Update(updated)
		} else {
			updated := registrationObj.DeepCopy()
			updated.Spec = *registrationObj.Spec.WithoutCheckNow()
			updated.Status.ActivationStatus.Valid = false
			updated, err := h.processOnlineActivation(updated)
			if err != nil {
				return h.setReconcilingCondition(registrationObj, err)
			}

			return updated, nil
		}
	}

	if !lastValidatedTS.IsZero() && time.Now().Sub(lastValidatedTS) < time.Hour {
		return registrationObj, nil
	}

	if registrationObj.Spec.Mode == v1.Online {
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

func (h *handler) setReconcilingCondition(registrationObj *v1.Registration, originalErr error) (*v1.Registration, error) {
	logrus.Info("[scc.registration-controller]: set reconciling condition")
	logrus.Error(originalErr)

	// TODO: actually set the message to something that makes sense based on the error
	v1.ResourceConditionFailure.ToK8sCondition().SetStatusBool(registrationObj, true)
	v1.ResourceConditionFailure.ToK8sCondition().SetError(registrationObj, "", originalErr)

	registrationObj, err := h.registrations.UpdateStatus(registrationObj)
	if err != nil {
		return registrationObj, errors.New(originalErr.Error() + err.Error())
	}

	return registrationObj, originalErr
}

func (h *handler) processOnlineActivation(registrationObj *v1.Registration) (*v1.Registration, error) {
	_ = h.sccCredentials.Refresh()
	regCode := suseconnect.FetchSccRegistrationCodeFrom(h.secrets, registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef)
	sccConnection := suseconnect.DefaultRancherConnection(h.sccCredentials.SccCredentials(), h.systemInfo)

	// TODO: remove override value - it's really just for testing
	identifier, version, arch := util.GetProductIdentifier("2.10.3")
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

	now := time.Now()
	logrus.Info("[scc.activation-controller]: Successfully registered activation")
	updated := registrationObj.DeepCopy()
	updated.Status.ActivationStatus.LastValidatedTS = now.UTC().Format(time.RFC3339)
	updated.Status.ActivationStatus.ValidUntilTS = now.Add(24 * time.Hour).UTC().Format(time.RFC3339)
	updated.Status.ActivationStatus.Valid = true
	// TODO: may need to unset the CheckNow on spec?
	return h.registrations.UpdateStatus(updated)
}

func (h *handler) processOfflineActivation(registrationObj *v1.Registration) (*v1.Registration, error) {
	return registrationObj, nil
}
