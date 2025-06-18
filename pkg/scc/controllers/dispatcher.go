package controllers

import (
	"github.com/rancher/rancher/pkg/scc/consts"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/util/jitterbug"
	"k8s.io/apimachinery/pkg/labels"
)

func setupCfg() *jitterbug.Config {
	// Configure jitter based daily revalidation trigger
	jitterbugConfig := jitterbug.Config{
		BaseInterval:    prodBaseCheckin,
		JitterMax:       3,
		JitterMaxScale:  time.Hour,
		PollingInterval: 9 * time.Minute,
	}
	if consts.IsDevMode() {
		jitterbugConfig = jitterbug.Config{
			BaseInterval:    devBaseCheckin,
			JitterMax:       10,
			JitterMaxScale:  time.Minute,
			PollingInterval: 9 * time.Second,
		}
	}
	return &jitterbugConfig
}

func (h *handler) RunLifecycleManager(
	cfg *jitterbug.Config,
) {
	// min jitter 20 hours
	jitterCheckin := jitterbug.NewJitterChecker(
		cfg,
		func(nextTrigger, strictDeadline time.Duration) (bool, error) {
			registrationsCacheList, err := h.registrationCache.List(labels.Everything())
			if err != nil {
				h.log.Errorf("Failed to list registrations: %v", err)
				return false, err
			}

			checkInWasTriggered := false
			for _, registrationObj := range registrationsCacheList {
				registrationHandler := h.prepareHandler(registrationObj)

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
					h.registrations.Enqueue(registrationObj.Name)
				}
			}

			return checkInWasTriggered, nil
		},
	)
	jitterCheckin.Start()
	jitterCheckin.Run()
}
