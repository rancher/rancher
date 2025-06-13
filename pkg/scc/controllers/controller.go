package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/scc/suseconnect/offlinerequest"
	"maps"
	"time"

	"github.com/rancher/rancher/pkg/scc/consts"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/scc/util/log"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	controllerID    = "prime-registration"
	prodBaseCheckin = time.Hour * 20
	prodMinCheckin  = prodBaseCheckin - (3 * time.Hour)
	devBaseCheckin  = time.Minute * 30
	devMinCheckin   = devBaseCheckin - (10 * time.Minute)
)

// SCCHandler Defines a common interface for online and offline operations
// IMPORTANT: All the `Reconcile*` methods modifies the object in memory but does NOT save it. The caller is responsible for saving the state.
// TODO: these that return `*v1.Registration` probably need to return a object set to use with apply
// TODO: That way we can use apply to update Registrations and Secrets related to them together.
type SCCHandler interface {
	// NeedsRegistration determines if the system requires initial SCC registration.
	NeedsRegistration(*v1.Registration) bool
	// NeedsActivation checks if the system requires activation with SCC.
	NeedsActivation(*v1.Registration) bool
	// ReadyForActivation checks if the system is ready for activation.
	ReadyForActivation(*v1.Registration) bool

	// PrepareForRegister preforms pre-registration steps
	PrepareForRegister(*v1.Registration) (*v1.Registration, error)
	// Register performs the initial system registration with SCC or creates an offline request.
	Register(*v1.Registration) (suseconnect.RegistrationSystemId, error)
	// PrepareRegisteredForActivation prepares the Registration object after successful registration.
	PrepareRegisteredForActivation(*v1.Registration) (*v1.Registration, error)
	// Activate activates the system with SCC or verifies an offline request.
	Activate(*v1.Registration) error
	// Keepalive provides a heartbeat to SCC and validates the system's status.
	Keepalive(registrationObj *v1.Registration) error
	// Deregister initiates the system's deregistration from SCC.
	Deregister() error

	// ReconcileRegisterError prepares the Registration object for error reconciliation after RegisterSystem fails.
	ReconcileRegisterError(*v1.Registration, error) *v1.Registration
	// ReconcileKeepaliveError prepares the Registration object for error reconciliation after Keepalive fails.
	ReconcileKeepaliveError(*v1.Registration, error) *v1.Registration
	// ReconcileActivateError prepares the Registration object for error reconciliation after Activate fails.
	ReconcileActivateError(*v1.Registration, error) *v1.Registration
}

type handler struct {
	ctx                context.Context
	log                *logrus.Entry
	registrations      registrationControllers.RegistrationController
	registrationCache  registrationControllers.RegistrationCache
	secrets            v1core.SecretController
	secretCache        v1core.SecretCache
	systemInfoExporter *systeminfo.InfoExporter
	systemNamespace    string
}

func Register(
	ctx context.Context,
	systemNamespace string,
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
		secretCache:        secrets.Cache(),
		systemInfoExporter: systemInfoExporter,
		systemNamespace:    systemNamespace,
	}

	controller.initIndexers()
	controller.initResolvers(ctx)
	secrets.OnChange(ctx, controllerID+"-secrets", controller.OnSecretChange)
	secrets.OnRemove(ctx, controllerID+"-secrets-remove", controller.OnSecretRemove)

	registrations.OnChange(ctx, controllerID, controller.OnRegistrationChange)
	registrations.OnRemove(ctx, controllerID+"-remove", controller.OnRegistrationRemove)

	cfg := setupCfg()
	go controller.RunLifecycleManager(cfg)
}

func (h *handler) prepareHandler(registrationObj *v1.Registration) SCCHandler {
	ref := registrationObj.ToOwnerRef()
	nameSuffixHash := registrationObj.Labels[consts.LabelNameSuffix]

	defaultLabels := map[string]string{
		consts.LabelSccHash:      registrationObj.Labels[consts.LabelSccHash],
		consts.LabelNameSuffix:   nameSuffixHash,
		consts.LabelSccManagedBy: controllerID,
	}

	if registrationObj.Spec.Mode == v1.RegistrationModeOffline {
		offlineRequestSecretName := consts.OfflineRequestSecretName(nameSuffixHash)
		return sccOfflineMode{
			registration:       registrationObj,
			log:                h.log.WithField("handler", "offline"),
			systemInfoExporter: h.systemInfoExporter,
			offlineSecrets: offlinerequest.New(
				h.systemNamespace,
				offlineRequestSecretName,
				consts.FinalizerSccCredentials,
				ref,
				h.secrets,
				h.secretCache,
				defaultLabels,
			),
		}
	}

	credsSecretName := consts.SCCCredentialsSecretName(nameSuffixHash)
	return sccOnlineMode{
		registration: registrationObj,
		log:          h.log.WithField("handler", "online"),
		sccCredentials: credentials.New(
			h.systemNamespace,
			credsSecretName,
			consts.FinalizerSccCredentials,
			ref,
			h.secrets,
			h.secretCache,
			defaultLabels,
		),
		systemInfoExporter: h.systemInfoExporter,
		secrets:            h.secrets,
		systemNamespace:    h.systemNamespace,
	}
}

func minResyncInterval() time.Time {
	now := time.Now()
	if util.VersionIsDevBuild() {
		return now.Add(-devMinCheckin)
	}
	return now.Add(-prodMinCheckin)
}

func (h *handler) OnSecretChange(name string, incomingObj *corev1.Secret) (*corev1.Secret, error) {
	if incomingObj == nil || incomingObj.DeletionTimestamp != nil {
		return incomingObj, nil
	}
	if h.isRancherEntrypointSecret(incomingObj) {
		if _, saltOk := incomingObj.GetLabels()[consts.LabelObjectSalt]; !saltOk {
			return h.prepareSecretSalt(incomingObj)
		}

		/** incomingSalt := incomingObj.GetLabels()[consts.LabelObjectSalt] */
		incomingHash := incomingObj.GetLabels()[consts.LabelSccHash]
		params, err := h.extraRegistrationParamsFromSecret(incomingObj)
		if err != nil {
			return incomingObj, fmt.Errorf("failed to extract registration params from secret %s/%s: %w", incomingObj.Namespace, incomingObj.Name, err)
		}

		if incomingHash == "" {
			// update secret with useful annotations & labels
			newSecret := incomingObj.DeepCopy()
			if newSecret.Annotations == nil {
				newSecret.Annotations = map[string]string{}
			}
			newSecret.Annotations[consts.LabelSccLastProcessed] = time.Now().Format(time.RFC3339)
			newSecret.Labels = params.Labels()

			savedSecret, updateErr := h.secrets.Update(newSecret)
			if updateErr != nil {
				h.log.Error("error applying metadata updates to default SCC registration secret")
				return nil, updateErr
			}

			return savedSecret, nil
		}

		// If secret hash has changed make sure that we submit objects that correspond to that hash
		// are cleaned up
		if incomingHash != params.contentHash {
			if cleanUpErr := h.cleanupRegistrationByHash(incomingHash); cleanUpErr != nil {
				h.log.Errorf("failed to cleanup registrations for hash %s: %v", incomingHash, cleanUpErr)
			}
		}

		// update secret with useful annotations & labels
		newSecret := incomingObj.DeepCopy()
		if newSecret.Annotations == nil {
			newSecret.Annotations = map[string]string{}
		}
		newSecret.Annotations[consts.LabelSccLastProcessed] = time.Now().Format(time.RFC3339)

		labels := incomingObj.Labels
		maps.Copy(labels, params.Labels())
		newSecret.Labels = labels

		_, updateErr := h.secrets.Update(newSecret)
		if updateErr != nil {
			h.log.Error("error applying metadata updates to default SCC registration secret")
			return nil, updateErr
		}

		// construct associated registration CRs
		registration, err := h.registrationFromSecretEntrypoint(params)
		if err != nil {
			return incomingObj, fmt.Errorf("failed to create registration from secret %s/%s: %w", incomingObj.Namespace, incomingObj.Name, err)
		}

		if createOrUpdateErr := h.createOrUpdate(registration); createOrUpdateErr != nil {
			h.log.Errorf("failed to create or update registration %s: %v", registration.Name, createOrUpdateErr)
			return incomingObj, fmt.Errorf("failed to create or update registration %s: %w", registration.Name, createOrUpdateErr)
		}
	}

	return incomingObj, nil
}

func (h *handler) createOrUpdate(reg *v1.Registration) error {
	if _, err := h.registrations.Get(reg.Name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			_, createErr := h.registrations.Create(reg)
			return createErr
		}
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentReg, err := h.registrations.Get(reg.Name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		preparedReg := currentReg.DeepCopy()
		preparedReg.Labels = reg.Labels
		preparedReg.Annotations = reg.Annotations
		preparedReg.Finalizers = reg.Finalizers
		preparedReg.Spec = reg.Spec

		_, updateErr := h.registrations.Update(preparedReg)
		return updateErr
	})
}

func (h *handler) cleanupRegistrationByHash(hash string) error {
	regs, err := h.registrationCache.GetByIndex(IndexRegistrationsBySccHash, hash)
	if err != nil {
		return err
	}

	for _, reg := range regs {
		if retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var remainingFin []string
			for _, finalizer := range reg.Finalizers {
				if finalizer != consts.FinalizerSccRegistration {
					remainingFin = append(remainingFin, finalizer)
				}
			}
			reg.Finalizers = remainingFin

			_, updateErr := h.registrations.Update(reg)
			return updateErr
		}); retryErr != nil {
			return retryErr
		}

		deleteErr := h.registrations.Delete(reg.Name, &metav1.DeleteOptions{})
		if apierrors.IsNotFound(deleteErr) {
			h.log.Debugf("Registration %s already deleted", reg.Name)
			continue
		}
		if deleteErr != nil {
			return fmt.Errorf("failed to delete registration %s: %w", reg.Name, err)
		}
	}
	return nil
}

func (h *handler) OnSecretRemove(name string, incomingObj *corev1.Secret) (*corev1.Secret, error) {
	if incomingObj == nil {
		return nil, nil
	}
	if incomingObj.Namespace != h.systemNamespace {
		h.log.Debugf("Secret %s/%s is not in SCC system namespace %s, skipping cleanup", incomingObj.Namespace, incomingObj.Name, h.systemNamespace)
		return incomingObj, nil
	}

	if h.isRancherEntrypointSecret(incomingObj) {
		hash, ok := incomingObj.Labels[consts.LabelSccHash]
		if !ok {
			return incomingObj, nil
		}

		if err := h.cleanupRegistrationByHash(hash); err != nil {
			h.log.Errorf("failed to cleanup registrations for hash %s: %v", hash, err)
			return nil, err
		}

		return incomingObj, nil
	}
	finalizers := incomingObj.GetFinalizers()
	for i, finalizer := range finalizers {
		// check if we are ready to remove the finalizer
		if finalizer == consts.FinalizerSccCredentials {
			refs := incomingObj.GetOwnerReferences()
			danglingRefs := 0
			for _, ref := range refs {
				if ref.APIVersion == v1.SchemeGroupVersion.String() &&
					ref.Kind == "Registration" {
					_, err := h.registrations.Get(ref.Name, metav1.GetOptions{})
					if apierrors.IsNotFound(err) {
						continue
					} else {
						danglingRefs++
					}
				}
			}
			if danglingRefs > 0 {
				h.log.Errorf("cannot remove finalizer %s from secret %s/%s, dangling references to Registration found", finalizer, incomingObj.Namespace, incomingObj.Name)
				return nil, fmt.Errorf("cannot remove finalizer %s from secret %s/%s, dangling references to Registration found", finalizer, incomingObj.Namespace, incomingObj.Name)
			}
			newSecret := incomingObj.DeepCopy()
			newSecret.SetFinalizers(append(finalizers[:i], finalizers[i+1:]...))
			logrus.Info("Removing finalizer from secret", newSecret.Name, "in namespace", newSecret.Namespace)
			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				_, err := h.secrets.Update(newSecret)
				return err
			}); err != nil {
				h.log.Errorf("failed to remove finalizer %s from secret %s/%s: %v", finalizer, incomingObj.Namespace, incomingObj.Name, err)
				return nil, fmt.Errorf("failed to remove finalizer %s from secret %s/%s: %w", finalizer, incomingObj.Namespace, incomingObj.Name, err)
			}
		}
	}

	return incomingObj, nil
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

	registrationHandler := h.prepareHandler(registrationObj)

	// Skip keepalive for anything activated within the last 20 hours
	if !registrationHandler.NeedsRegistration(registrationObj) &&
		!registrationHandler.NeedsActivation(registrationObj) &&
		registrationObj.Spec.SyncNow == nil {
		if !registrationObj.Status.ActivationStatus.LastValidatedTS.IsZero() &&
			registrationObj.Status.ActivationStatus.LastValidatedTS.Time.After(minResyncInterval()) {
			return registrationObj, nil
		}
	}

	// Only on the first time an object passes through here should it need to be registered
	// The logical default condition should always be to try activation, unless we know it's not registered.
	if registrationHandler.NeedsRegistration(registrationObj) {
		progressingObj := registrationObj.DeepCopy()
		if !registrationObj.HasCondition(v1.ResourceConditionProgressing) || v1.ResourceConditionProgressing.IsFalse(registrationObj) {
			// Set object to progressing
			progressingUpdateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var err error
				v1.ResourceConditionProgressing.True(progressingObj)
				progressingObj, err = h.registrations.UpdateStatus(progressingObj)
				return err
			})
			if progressingUpdateErr != nil {
				return registrationObj, progressingUpdateErr
			}
		}

		regForAnnounce := progressingObj.DeepCopy()
		preparedForRegister, prepareErr := registrationHandler.PrepareForRegister(regForAnnounce)
		if prepareErr != nil {
			return progressingObj, prepareErr
		}
		regForAnnounce, updateErr := h.registrations.UpdateStatus(preparedForRegister)
		if updateErr != nil {
			return progressingObj, prepareErr
		}

		announcedSystemId, registerErr := registrationHandler.Register(regForAnnounce)
		if registerErr != nil {
			// reconcile state
			var reconciledReg *v1.Registration
			reconcileErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var retryErr, reconcileUpdateErr error
				registrationObj, retryErr = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
				if retryErr != nil {
					return retryErr
				}
				prepareObj := registrationObj.DeepCopy()
				prepareObj = registrationHandler.ReconcileRegisterError(prepareObj, registerErr)

				reconciledReg, reconcileUpdateErr = h.registrations.Update(prepareObj)
				return reconcileUpdateErr
			})

			err := fmt.Errorf("registration failed: %w", registerErr)
			if reconcileErr != nil {
				err = fmt.Errorf("registration failed with additional errors: %w, %w", err, reconcileErr)
			}

			return reconciledReg, err
		}

		setSystemId := false
		switch announcedSystemId {
		case suseconnect.OfflineRegistrationSystemId:
			h.log.Debugf("SCC system ID cannot be known for offline until activation")
		case suseconnect.KeepAliveRegistrationSystemId:
			h.log.Debugf("register system handled via keepalive")
			announcedSystemId = suseconnect.RegistrationSystemId(*registrationObj.Status.SCCSystemId)
		default:
			h.log.Debugf("Annoucned System ID: %v", announcedSystemId)
			setSystemId = true
		}

		var prepareError error
		// Prepare the Registration for Activation phase
		if setSystemId {
			regForAnnounce.Status.SCCSystemId = announcedSystemId.Ptr()
		}
		regForAnnounce, prepareError = registrationHandler.PrepareRegisteredForActivation(regForAnnounce)
		if prepareError != nil {
			return registrationObj, prepareError
		}
		regForAnnounce.Status.RegistrationProcessedTS = &metav1.Time{
			Time: time.Now(),
		}

		announced, registerUpdateErr := h.registrations.UpdateStatus(regForAnnounce)
		if registerUpdateErr != nil {
			return registrationObj, registerUpdateErr
		}

		return announced, nil
	}

	if registrationHandler.NeedsActivation(registrationObj) {
		if !registrationHandler.ReadyForActivation(registrationObj) {
			h.log.Debugf("registration needs to be activated, but not yet ready; %v", registrationObj)
			return registrationObj, nil
		}
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
				prepareObj := registrationObj.DeepCopy()
				prepareObj = registrationHandler.ReconcileActivateError(prepareObj, activationErr)

				reconciledReg, reconcileUpdateErr = h.registrations.Update(prepareObj)
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
			v1.RegistrationConditionSccUrlReady.True(activated) // TODO: online only?
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

	keepaliveErr := registrationHandler.Keepalive(registrationObj)
	if keepaliveErr != nil {
		var reconciledReg *v1.Registration
		reconcileErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var retryErr, reconcileUpdateErr error
			registrationObj, retryErr = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
			if retryErr != nil {
				return retryErr
			}
			preparedObj := registrationHandler.ReconcileKeepaliveError(registrationObj, keepaliveErr)

			reconciledReg, reconcileUpdateErr = h.registrations.Update(preparedObj)
			return reconcileUpdateErr
		})

		return reconciledReg, reconcileErr
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

// do we want a finalizer for preventing deregister from failing then deleting object?
func (h *handler) OnRegistrationRemove(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, nil
	}

	regHandler := h.prepareHandler(registrationObj)
	deRegErr := regHandler.Deregister()
	if deRegErr != nil {
		h.log.Warn(deRegErr)
	}

	// TODO: owner finalizers handled here
	// (alex) :  I don't think this is neded
	err := h.registrations.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return registrationObj, err
	}

	return nil, nil
}
