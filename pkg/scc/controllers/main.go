package controllers

import (
	"context"
	"errors"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/controllers/activation"
	"github.com/rancher/rancher/pkg/scc/controllers/registration"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/util"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	registrations.OnChange(ctx, "registrations", controller.OnRegistrationChange)
	registrations.OnRemove(ctx, "registrations", controller.OnRegistrationRemove)
	// TODO: EnqueueAfter - revalidate every 24 hours
	// Ex: https://github.com/rancher/rancher/blob/d6b40c3acd945f0c8fe463ff96d144561c9640c3/pkg/controllers/dashboard/helm/repo.go#L95
}

func (h *handler) OnRegistrationChange(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, nil
	}

	if !h.isServerUrlReady() {
		logrus.Info("[scc.registration-controller]: Server URL not set")
		return registrationObj, errors.New("no server url found in the system info")
	}

	// Only on the first time an object passes through here should it need to be registered
	// The logical default condition should always be to try activation, unless we know it's not registered.
	// That is why these conditions may look a bit odd, as it helps ensure registration logic is used as needed.
	if registrationObj.Status.RegistrationStatus.RequestProcessedTS == nil || registrationObj.Status.RegistrationStatus.RequestProcessedTS.IsZero() ||
		!registrationObj.HasCondition(v1.RegistrationConditionSccUrlReady) ||
		!registrationObj.HasCondition(v1.RegistrationConditionAnnounced) {
		registrationHandler := registration.New(
			h.ctx,
			h.registrations,
			h.secrets,
			h.sccCredentials,
			h.systemInfo,
		)

		return registrationHandler.Call(name, registrationObj)
	}

	registrationHandler := activation.New(
		h.ctx,
		h.registrations,
		h.secrets,
		h.sccCredentials,
		h.systemInfo,
	)

	return registrationHandler.Call(name, registrationObj)
}

func (h *handler) isServerUrlReady() bool {
	return h.systemInfo.ServerUrl() != ""
}

func (h *handler) OnRegistrationRemove(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, nil
	}

	// For online mode, call deregister
	if registrationObj.Spec.Mode == v1.Online {
		_ = h.sccCredentials.Refresh()
		sccConnection := suseconnect.DefaultRancherConnection(h.sccCredentials.SccCredentials(), h.systemInfo)
		err := sccConnection.Deregister()
		if err != nil {
			return nil, err
		}

		// Delete SCC credentials after successful Deregister
		credErr := h.sccCredentials.Remove()
		if credErr != nil {
			return nil, credErr
		}
	}

	err := h.registrations.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return registrationObj, err
	}

	return nil, nil
}
