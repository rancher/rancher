package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/scc/util/jitterbug"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"github.com/rancher/wrangler/v3/pkg/apply"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

const (
	controllerID    = "prime-registration"
	prodBaseCheckin = time.Hour * 20
	prodMinCheckin  = prodBaseCheckin - (3 * time.Hour)
	devBaseCheckin  = time.Minute * 30
	devMinCheckin   = devBaseCheckin - (10 * time.Minute)
)

// TODO: these that return `*v1.Registration` probably need to return a object set to use with apply
// TODO: That way we can use apply to update Registrations and Secrets related to them together.
// IMPORTANT: All the `Reconcile*` methods modifies the object in memory but does NOT save it. The caller is responsible for saving the state.
type SCCHandler interface {
	// NeedsRegistration determines if the system requires initial SCC registration.
	NeedsRegistration(*v1.Registration) bool
	// RegisterSystem performs the initial system registration with SCC or creates an offline request.
	RegisterSystem(*v1.Registration) (suseconnect.RegistrationSystemId, error)
	// ReconcileRegisterSystemError prepares the Registration object for error reconciliation after RegisterSystem fails.
	ReconcileRegisterSystemError(*v1.Registration, error) *v1.Registration
	// PrepareRegisteredSystem prepares the Registration object after successful registration.
	PrepareRegisteredSystem(*v1.Registration) *v1.Registration
	// NeedsActivation checks if the system requires activation with SCC.
	NeedsActivation(*v1.Registration) bool
	// Activate activates the system with SCC or verifies an offline request.
	Activate(*v1.Registration) error
	// ReconcileActivateError prepares the Registration object for error reconciliation after Activate fails.
	ReconcileActivateError(*v1.Registration, error) *v1.Registration
	// Keepalive provides a heartbeat to SCC and validates the system's status.
	Keepalive(registrationObj *v1.Registration) error
	// ReconcileKeepaliveError prepares the Registration object for error reconciliation after Keepalive fails.
	ReconcileKeepaliveError(*v1.Registration, error) *v1.Registration
	// Deregister initiates the system's deregistration from SCC.
	Deregister() error
}

type handler struct {
	apply              apply.Apply
	ctx                context.Context
	log                *logrus.Entry
	registrations      registrationControllers.RegistrationController
	registrationCache  registrationControllers.RegistrationCache
	secrets            v1core.SecretController
	sccCredentials     *credentials.CredentialSecretsAdapter
	systemInfoExporter *systeminfo.InfoExporter
}

func Register(
	ctx context.Context,
	wApply apply.Apply,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	systemInfoExporter *systeminfo.InfoExporter,
) {
	controller := &handler{
		apply: wApply.
			WithCacheTypes(registrations).
			WithSetID(controllerID).
			WithSetOwnerReference(true, false),
		log:                log.NewControllerLogger("registration-controller"),
		ctx:                ctx,
		registrations:      registrations,
		registrationCache:  registrations.Cache(),
		secrets:            secrets,
		sccCredentials:     credentials.New(secrets),
		systemInfoExporter: systemInfoExporter,
	}

	registrations.OnChange(ctx, controllerID, controller.OnRegistrationChange)
	registrations.OnRemove(ctx, controllerID+"remove", controller.OnRegistrationRemove)
	relatedresource.Watch(ctx, controllerID+"-secrets",
		relatedresource.
			OwnerResolver(true, v1.SchemeGroupVersion.String(), v1.RegistrationResourceName),
		secrets,
		registrations,
	)

	// Configure jitter based daily revalidation trigger
	jitterbugConfig := jitterbug.Config{
		BaseInterval:    prodBaseCheckin,
		JitterMax:       3,
		JitterMaxScale:  time.Hour,
		PollingInterval: 9 * time.Minute,
	}
	if util.VersionIsDevBuild() {
		jitterbugConfig = jitterbug.Config{
			BaseInterval:    devBaseCheckin,
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
			log:                h.log.WithField("handler", "offline"),
			systemInfoExporter: h.systemInfoExporter,
			secrets:            h.secrets,
		}
	}
	return sccOnlineMode{
		log:                h.log.WithField("handler", "online"),
		sccCredentials:     h.sccCredentials,
		systemInfoExporter: h.systemInfoExporter,
		secrets:            h.secrets,
	}
}

func minResyncInterval() time.Time {
	now := time.Now()
	if util.VersionIsDevBuild() {
		return now.Add(-devMinCheckin)
	}
	return now.Add(-prodMinCheckin)
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

	// Skip keepalive for anything activated within the last 20 hours
	if !registrationHandler.NeedsRegistration(registrationObj) &&
		!registrationHandler.NeedsActivation(registrationObj) &&
		registrationObj.Spec.SyncNow == nil {
		if registrationObj.Status.ActivationStatus.LastValidatedTS.Time.After(minResyncInterval()) {
			return registrationObj, nil
		}
	}

	// Only on the first time an object passes through here should it need to be registered
	// The logical default condition should always be to try activation, unless we know it's not registered.
	if registrationHandler.NeedsRegistration(registrationObj) {
		// Set object to progressing
		progressingUpdateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var err error
			progressingObj := registrationObj.DeepCopy()
			v1.ResourceConditionProgressing.True(progressingObj)
			progressingObj, err = h.registrations.UpdateStatus(progressingObj)
			return err
		})
		if progressingUpdateErr != nil {
			return registrationObj, progressingUpdateErr
		}

		announcedSystemId, registerErr := registrationHandler.RegisterSystem(registrationObj)
		if registerErr != nil {
			// reconcile state
			var reconciledReg *v1.Registration
			reconcileErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var retryErr, reconcileUpdateErr error
				registrationObj, retryErr = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
				if retryErr != nil {
					return retryErr
				}
				reconciledObj := registrationHandler.ReconcileRegisterSystemError(registrationObj, registerErr)

				reconciledReg, reconcileUpdateErr = h.registrations.Update(reconciledObj)
				return reconcileUpdateErr
			})

			err := fmt.Errorf("registration failed: %w", registerErr)
			if reconcileErr != nil {
				err = fmt.Errorf("registration failed with additional errors: %w, %w", err, reconcileErr)
			}

			return reconciledReg, err
		}

		switch announcedSystemId {
		case suseconnect.KeepAliveRegistrationSystemId:
			announcedSystemId = suseconnect.RegistrationSystemId(registrationObj.Status.SCCSystemId)
		default:
			h.log.Debugf("Annoucned System ID: %v", announcedSystemId)
		}

		// Prepare the Registration for Activation phase
		var announced *v1.Registration
		registerUpdateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var err error
			registrationObj, err = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			updatingObj := registrationHandler.PrepareRegisteredSystem(registrationObj)
			updatingObj.Status.RegistrationProcessedTS = &metav1.Time{
				Time: time.Now(),
			}

			announced, err = h.registrations.UpdateStatus(updatingObj)

			return err
		})
		if registerUpdateErr != nil {
			return registrationObj, registerUpdateErr
		}

		// Upon successful registration the processed TS should be set, so when it is enqueue for activation
		if announced.Status.RegistrationProcessedTS != nil {
			h.registrations.Enqueue(registrationObj.Name)
		}

		return announced, nil
	}

	if registrationHandler.NeedsActivation(registrationObj) {
		activationErr := registrationHandler.Activate(registrationObj)
		// reconcile error state - must be able to handle Auth errors (or other SCC sourced errors)
		if activationErr != nil {
			var reconciledReg *v1.Registration
			reconcileErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var retryErr, reconcileUpdateErr error
				registrationObj, retryErr = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
				if retryErr != nil {
					return retryErr
				}
				preparedObj := registrationHandler.ReconcileActivateError(registrationObj, activationErr)

				reconciledReg, reconcileUpdateErr = h.registrations.Update(preparedObj)
				return reconcileUpdateErr
			})

			err := fmt.Errorf("activation failed: %w", activationErr)
			if reconcileErr != nil {
				err = fmt.Errorf("activation failed with additional errors: %w, %w", err, reconcileErr)
			}

			return reconciledReg, err
		}

		var activatedObj *v1.Registration
		activatedUpdateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var retryErr, updateErr error
			registrationObj, retryErr = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
			if retryErr != nil {
				return retryErr
			}

			activated := registrationObj.DeepCopy()
			v1.RegistrationConditionSccUrlReady.True(activated)
			v1.RegistrationConditionActivated.True(activated)
			v1.ResourceConditionProgressing.False(activated)
			v1.ResourceConditionReady.True(activated)
			v1.ResourceConditionDone.True(activated)
			activated.Status.ActivationStatus.LastValidatedTS = &metav1.Time{
				Time: time.Now(),
			}
			activated.Status.ActivationStatus.Activated = true
			activatedObj, updateErr = h.registrations.UpdateStatus(activated)
			return updateErr
		})
		if activatedUpdateErr != nil {
			return registrationObj, activatedUpdateErr
		}

		return activatedObj, nil
	}

	// Handle what to do when CheckNow is used...
	if registrationObj.Spec.SyncNow != nil && *registrationObj.Spec.SyncNow {
		if registrationObj.Spec.Mode == v1.RegistrationModeOffline {
			updated := registrationObj.DeepCopy()
			// TODO(o&b): When offline calls this it should immediately sync the OfflineRegistrationRequest secret content
			updated.Spec = *registrationObj.Spec.WithoutSyncNow()
			return h.registrations.Update(updated)
		} else {
			updated := registrationObj.DeepCopy()
			updated.Spec = *registrationObj.Spec.WithoutSyncNow()
			updated.Status.ActivationStatus.Activated = false
			updated.Status.ActivationStatus.LastValidatedTS = &metav1.Time{}
			v1.ResourceConditionProgressing.True(updated)
			v1.ResourceConditionReady.False(updated)
			v1.ResourceConditionDone.False(updated)

			var err error
			updated, err = h.registrations.UpdateStatus(updated)

			updated.Spec = *registrationObj.Spec.WithoutSyncNow()
			updated, err = h.registrations.Update(updated)
			return updated, err
		}
	}

	err := registrationHandler.Keepalive(registrationObj)
	if err != nil {
		// reconcile state
		return nil, err
	}

	var heartbeatUpdated *v1.Registration
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
		// TODO: make sure we set Activated condition and add "ValidUntilTS" to that status
		updated.Status.ActivationStatus.Activated = true
		heartbeatUpdated, err = h.registrations.UpdateStatus(updated)
		return err
	})
	if updateErr != nil {
		return nil, updateErr
	}

	return heartbeatUpdated, nil
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
