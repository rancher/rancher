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
	"github.com/rancher/rancher/pkg/scc/util/jitterbug"
	"github.com/rancher/rancher/pkg/version"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"time"
)

type handler struct {
	ctx               context.Context
	registrations     registrationControllers.RegistrationController
	registrationCache registrationControllers.RegistrationCache
	secrets           v1core.SecretController
	sccCredentials    *credentials.CredentialSecretsAdapter
	systemInfo        *util.RancherSystemInfo
}

func Register(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	systemInfo *util.RancherSystemInfo,
) {
	controller := &handler{
		ctx:               ctx,
		registrations:     registrations,
		registrationCache: registrations.Cache(),
		secrets:           secrets,
		sccCredentials:    credentials.New(secrets),
		systemInfo:        systemInfo,
	}

	registrations.OnChange(ctx, "registration-controller", controller.OnRegistrationChange)
	registrations.OnRemove(ctx, "registration-controller", controller.OnRegistrationRemove)

	// Configure jitter based daily revalidation trigger
	jitterbugConfig := jitterbug.Config{
		BaseInterval:    20 * time.Hour,
		JitterMax:       3,
		JitterMaxScale:  time.Hour,
		PollingInterval: 9 * time.Minute,
	}
	if version.IsDevBuild() {
		jitterbugConfig = jitterbug.Config{
			BaseInterval:    8 * time.Hour,
			JitterMax:       30,
			JitterMaxScale:  time.Minute,
			PollingInterval: 9 * time.Second,
		}
	}
	jitterCheckin := jitterbug.NewJitterChecker(
		&jitterbugConfig,
		func(dailyTriggerTime, strictDeadline time.Duration) (bool, error) {
			registrationsCacheList, err := controller.registrationCache.List(labels.Everything())
			if err != nil {
				logrus.Errorf("Failed to list registrations: %v", err)
				return false, err
			}

			checkInWasTriggered := false
			for _, registrationObj := range registrationsCacheList {
				// Always skip offline mode registrations, or Registrations that haven't progressed to activation
				if registrationObj.Spec.Mode == v1.Offline ||
					needsRegistration(registrationObj) {
					continue
				}

				lastValidated := registrationObj.Status.ActivationStatus.LastValidatedTS
				timeSinceLastValidation := time.Since(lastValidated.Time)
				// If the time since last validation is after the daily trigger (which includes jitter), we revalidate.
				// Also, ensure that when a registration is over the strictDeadline it is checked.
				if timeSinceLastValidation >= dailyTriggerTime || timeSinceLastValidation >= strictDeadline {
					checkInWasTriggered = true
					// Potentially this we should update the registration here too?
					// Mark it as `checkNow` or something similar? (not sure enqueue alone will cause the check to happen)
					registrations.Enqueue(registrationObj.Name)
				}
			}

			return checkInWasTriggered, nil
		},
	)
	jitterCheckin.Start()
	go jitterCheckin.Run()
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
	if needsRegistration(registrationObj) {
		registrationHandler := registration.New(
			h.ctx,
			h.registrations,
			h.secrets,
			h.sccCredentials,
			h.systemInfo,
		)

		return registrationHandler.Call(name, registrationObj)
	}

	// Due to the above noted choice, this means if the Registration becomes invalid outside of Rancher
	// Then the activation handler is what should deal with reconciling the state when that happens
	activationHandler := activation.New(
		h.ctx,
		h.registrations,
		h.secrets,
		h.sccCredentials,
		h.systemInfo,
	)

	return activationHandler.Call(name, registrationObj)
}

func needsRegistration(obj *v1.Registration) bool {
	return obj.Status.RegistrationProcessedTS == nil || obj.Status.RegistrationProcessedTS.IsZero() ||
		!obj.HasCondition(v1.RegistrationConditionSccUrlReady) ||
		!obj.HasCondition(v1.RegistrationConditionAnnounced)
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
