package controllers

import (
	"context"
	"errors"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/scc/util/jitterbug"
	"github.com/rancher/rancher/pkg/scc/util/log"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"time"
)

const (
	prodMinCheckin = time.Hour * 20
	devMinCheckin  = time.Minute * 30
)

type SCCHandler interface {
	NeedsRegistration(*v1.Registration) bool
	RegisterSystem(*v1.Registration) (*v1.Registration, error) // Equal to first time registration w/ SCC, or Offline Request creation
	NeedsActivation(*v1.Registration) bool
	Activate(*v1.Registration) (*v1.Registration, error)  // Equal to activating with SCC, or verifying offline Request
	Keepalive(*v1.Registration) (*v1.Registration, error) // Provides a heartbeat to SCC and validates status
	Deregister() error
}

type handler struct {
	log                log.StructuredLogger
	ctx                context.Context
	registrations      registrationControllers.RegistrationController
	registrationCache  registrationControllers.RegistrationCache
	secrets            v1core.SecretController
	sccCredentials     *credentials.CredentialSecretsAdapter
	systemInfoExporter *systeminfo.InfoExporter
}

func Register(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	systemInfoExporter *systeminfo.InfoExporter,
) {
	controller := &handler{
		log:                log.NewControllerLogger("registration-controller"),
		ctx:                ctx,
		registrations:      registrations,
		registrationCache:  registrations.Cache(),
		secrets:            secrets,
		sccCredentials:     credentials.New(secrets),
		systemInfoExporter: systemInfoExporter,
	}

	registrations.OnChange(ctx, "registration-controller", controller.OnRegistrationChange)
	registrations.OnRemove(ctx, "registration-controller", controller.OnRegistrationRemove)

	// Configure jitter based daily revalidation trigger
	jitterbugConfig := jitterbug.Config{
		BaseInterval:    prodMinCheckin,
		JitterMax:       3,
		JitterMaxScale:  time.Hour,
		PollingInterval: 9 * time.Minute,
	}
	if util.VersionIsDevBuild() {
		jitterbugConfig = jitterbug.Config{
			BaseInterval:    devMinCheckin,
			JitterMax:       10,
			JitterMaxScale:  time.Minute,
			PollingInterval: 9 * time.Second,
		}
	}
	jitterCheckin := jitterbug.NewJitterChecker(
		&jitterbugConfig,
		func(nextTrigger, strictDeadline time.Duration) (bool, error) {
			registrationsCacheList, err := controller.registrationCache.List(labels.Everything())
			if err != nil {
				controller.log.Errorf("Failed to list registrations: %v", err)
				return false, err
			}

			checkInWasTriggered := false
			for _, registrationObj := range registrationsCacheList {
				registrationHandler := controller.prepareHandler(registrationObj.Spec.Mode)

				// Always skip offline mode registrations, or Registrations that haven't progressed to activation
				if registrationObj.Spec.Mode == v1.RegistrationModeOffline ||
					registrationHandler.NeedsRegistration(registrationObj) ||
					registrationObj.Status.ActivationStatus.LastValidatedTS.IsZero() {
					continue
				}

				lastValidated := registrationObj.Status.ActivationStatus.LastValidatedTS

				timeSinceLastValidation := time.Since(lastValidated.Time)
				// If the time since last validation is after the daily trigger (which includes jitter), we revalidate.
				// Also, ensure that when a registration is over the strictDeadline it is checked.
				if timeSinceLastValidation >= nextTrigger || timeSinceLastValidation >= strictDeadline {
					checkInWasTriggered = true
					// TODO (o&b): 95% sure that enqueue alone won't be good enough based on other controller logic.
					// Either we need to adjust that controller logic so enqueue alone is enough, or use `CheckNow`.
					// Seems check now is most simple as it reuses controller logic
					registrations.Enqueue(registrationObj.Name)
				}
			}

			return checkInWasTriggered, nil
		},
	)
	jitterCheckin.Start()
	go jitterCheckin.Run()
}

func (h *handler) prepareHandler(mode v1.RegistrationMode) SCCHandler {
	if mode == v1.RegistrationModeOffline {
		return sccOfflineMode{
			log: h.log.WithField("handler", "offline"),
		}
	}
	return sccOnlineMode{
		log:                h.log.WithField("handler", "online"),
		registrations:      h.registrations,
		sccCredentials:     h.sccCredentials,
		systemInfoExporter: h.systemInfoExporter,
		secrets:            h.secrets,
	}
}

func (h *handler) OnRegistrationChange(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, nil
	}

	if !systeminfo.IsServerUrlReady() {
		h.log.Info("Server URL not set")
		return registrationObj, errors.New("no server url found in the system info")
	}

	if v1.ResourceConditionFailure.IsTrue(registrationObj) {
		return registrationObj, errors.New("registration has failed status; create a new one to retry")
	}

	registrationHandler := h.prepareHandler(registrationObj.Spec.Mode)
	if !registrationHandler.NeedsRegistration(registrationObj) &&
		!registrationHandler.NeedsActivation(registrationObj) &&
		registrationObj.Spec.CheckNow == nil {
		// Skip keepalive for anything activated within the last 20 hours
		recheckTime := time.Now().Add(-prodMinCheckin)
		if util.VersionIsDevBuild() {
			recheckTime = time.Now().Add(-devMinCheckin)
		}
		if registrationObj.Status.ActivationStatus.LastValidatedTS.Time.After(recheckTime) {
			return registrationObj, nil
		}
	}

	// Only on the first time an object passes through here should it need to be registered
	// The logical default condition should always be to try activation, unless we know it's not registered.
	if registrationHandler.NeedsRegistration(registrationObj) {
		announced, err := registrationHandler.RegisterSystem(registrationObj)
		if err != nil {
			// reconcile state
			return nil, err
		}

		// Upon successful registration the processed TS should be set, so when it is enqueue for activation
		if registrationObj.Status.RegistrationProcessedTS != nil {
			h.registrations.Enqueue(registrationObj.Name)
		}

		return announced, nil
	}

	if registrationHandler.NeedsActivation(registrationObj) {
		activated, err := registrationHandler.Activate(registrationObj)
		if err != nil {
			// reconcile state
			return nil, err
		}

		return activated, nil
	}

	// Handle what to do when CheckNow is used...
	if registrationObj.Spec.CheckNow != nil && *registrationObj.Spec.CheckNow {
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

	heartbeat, err := registrationHandler.Keepalive(registrationObj)
	if err != nil {
		// reconcile state
		return nil, err
	}

	return heartbeat, nil
}

func (h *handler) OnRegistrationRemove(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, nil
	}

	regHandler := h.prepareHandler(registrationObj.Spec.Mode)
	deRegErr := regHandler.Deregister()
	if deRegErr != nil {
		h.log.Warn(deRegErr)
	}

	err := h.registrations.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return registrationObj, err
	}

	return nil, nil
}
