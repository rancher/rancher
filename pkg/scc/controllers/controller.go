package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/scc/controllers/repos"
	"github.com/rancher/rancher/pkg/scc/systeminfo/secret"
	"github.com/rancher/rancher/pkg/scc/types"
	"github.com/rancher/rancher/pkg/telemetry"
	"maps"
	"slices"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/controllers/common"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/suseconnect/offline"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util/log"

	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	// PrepareActivatedForKeepalive prepares an Activated Registration for future keepalive
	PrepareActivatedForKeepalive(*v1.Registration) (*v1.Registration, error)
	// Keepalive provides a heartbeat to SCC and validates the system's status.
	Keepalive(registrationObj *v1.Registration) error
	// PrepareKeepaliveSucceeded completes any necessary steps after successful keepalive
	PrepareKeepaliveSucceeded(*v1.Registration) (*v1.Registration, error)
	// Deregister initiates the system's deregistration from SCC.
	Deregister() error

	// ReconcileRegisterError prepares the Registration object for error reconciliation after RegisterSystem fails.
	ReconcileRegisterError(*v1.Registration, error, types.RegistrationPhase) *v1.Registration
	// ReconcileKeepaliveError prepares the Registration object for error reconciliation after Keepalive fails.
	ReconcileKeepaliveError(*v1.Registration, error) *v1.Registration
	// ReconcileActivateError prepares the Registration object for error reconciliation after Activate fails.
	ReconcileActivateError(*v1.Registration, error, types.ActivationPhase) *v1.Registration
}

type handler struct {
	ctx                  context.Context
	log                  *logrus.Entry
	registrations        registrationControllers.RegistrationController
	registrationCache    registrationControllers.RegistrationCache
	secretRepo           repos.SecretRepo
	secrets              v1core.SecretController
	secretCache          v1core.SecretCache
	systemInfoExporter   *systeminfo.InfoExporter
	metricsSecretManager *secret.MetricsSecretManager
	systemNamespace      string
}

func Register(
	ctx context.Context,
	systemNamespace string,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	rancherTelemetry telemetry.TelemetryGatherer,
	systemInfoProvider *systeminfo.InfoProvider,
) {
	secretsRepo := repos.SecretRepo{
		Secrets:      secrets,
		SecretsCache: secrets.Cache(),
	}

	systemInfoExporter := systeminfo.NewInfoExporter(
		systemInfoProvider,
		rancherTelemetry,
		log.NewLog().WithField("subcomponent", "systeminfo-exporter"),
		secret.New(systemNamespace, &secretsRepo),
	)

	controller := &handler{
		log:                log.NewControllerLogger("registration-controller"),
		ctx:                ctx,
		registrations:      registrations,
		registrationCache:  registrations.Cache(),
		secretRepo:         secretsRepo,
		secrets:            secrets,
		secretCache:        secrets.Cache(),
		systemInfoExporter: systemInfoExporter,
		systemNamespace:    systemNamespace,
	}

	controller.initIndexers()
	controller.initResolvers(ctx)
	// secrets.OnChange(ctx, controllerID+"-secrets", controller.OnSecretChange)
	scopedOnChange(ctx, controllerID+"-secrets", systemNamespace, secrets, controller.OnSecretChange)
	scopedOnRemove(ctx, controllerID+"-secrets-remove", systemNamespace, secrets, controller.OnSecretRemove)

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
		offlineCertSecretName := consts.OfflineCertificateSecretName(nameSuffixHash)
		return sccOfflineMode{
			registration:       registrationObj,
			log:                h.log.WithField("regHandler", "offline"),
			systemInfoExporter: h.systemInfoExporter,
			offlineSecrets: offline.New(
				h.systemNamespace,
				offlineRequestSecretName,
				offlineCertSecretName,
				ref,
				h.secrets,
				h.secretCache,
				defaultLabels,
			),
			systemNamespace: h.systemNamespace,
		}
	}

	credsSecretName := consts.SCCCredentialsSecretName(nameSuffixHash)
	return sccOnlineMode{
		registration: registrationObj,
		log:          h.log.WithField("regHandler", "online"),
		sccCredentials: credentials.New(
			h.systemNamespace,
			credsSecretName,
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

func (h *handler) OnSecretChange(name string, incomingObj *corev1.Secret) (*corev1.Secret, error) {
	if incomingObj == nil || incomingObj.DeletionTimestamp != nil {
		return incomingObj, nil
	}
	if h.isRancherEntrypointSecret(incomingObj) {
		if _, saltOk := incomingObj.GetLabels()[consts.LabelObjectSalt]; !saltOk {
			return h.prepareSecretSalt(incomingObj)
		}

		incomingNameHash := incomingObj.GetLabels()[consts.LabelNameSuffix]
		incomingContentHash := incomingObj.GetLabels()[consts.LabelSccHash]
		params, err := extractRegistrationParamsFromSecret(incomingObj)
		if err != nil {
			return incomingObj, fmt.Errorf("failed to extract registration params from secret %s/%s: %w", incomingObj.Namespace, incomingObj.Name, err)
		}

		if incomingContentHash == "" {
			h.log.Info("incoming hash empty, prepare it")
			// update secret with useful annotations & labels
			newSecret := incomingObj.DeepCopy()
			if newSecret.Annotations == nil {
				newSecret.Annotations = map[string]string{}
			}
			newSecret.Annotations[consts.LabelSccLastProcessed] = time.Now().Format(time.RFC3339)
			maps.Copy(newSecret.Labels, params.Labels())

			_, updateErr := h.secretRepo.RetryingPatchUpdate(incomingObj, newSecret)
			if updateErr != nil {
				h.log.Error("error applying metadata updates to default SCC registration secret")
				return nil, updateErr
			}

			return incomingObj, nil
		}

		// If secret hash has changed make sure that we submit objects that correspond to that hash
		// are cleaned up
		if incomingNameHash != params.nameId {
			h.log.Info("must cleanup existing registration managed by secret")
			if cleanUpErr := h.cleanupRegistrationByHash(hashCleanupRequest{
				incomingNameHash,
				NameHash,
			}); cleanUpErr != nil {
				h.log.Errorf("failed to cleanup registrations for hash %s: %v", incomingNameHash, cleanUpErr)
				return incomingObj, cleanUpErr
			}
		}

		if incomingContentHash != params.contentHash {
			h.log.Info("must cleanup existing registration managed by secret")
			if cleanUpErr := h.cleanupRelatedSecretsByHash(incomingContentHash); cleanUpErr != nil {
				h.log.Errorf("failed to cleanup registrations for hash %s: %v", incomingNameHash, cleanUpErr)
				return incomingObj, cleanUpErr
			}
		}

		h.log.Info("create or update registration managed by secret")

		// update secret with useful annotations & labels
		newSecret := incomingObj.DeepCopy()
		if newSecret.Annotations == nil {
			newSecret.Annotations = map[string]string{}
		}
		newSecret.Annotations[consts.LabelSccLastProcessed] = time.Now().Format(time.RFC3339)

		labels := incomingObj.Labels
		maps.Copy(labels, params.Labels())
		newSecret.Labels = labels

		if _, err := h.secretRepo.RetryingPatchUpdate(incomingObj, newSecret); err != nil {
			return incomingObj, err
		}

		if params.regType == v1.RegistrationModeOffline && params.hasOfflineCertData {
			offlineCertSecret, err := h.offlineCertFromSecretEntrypoint(params)
			if err != nil {
				return incomingObj, err
			}

			if _, err := h.secretRepo.CreateOrUpdateSecret(offlineCertSecret); err != nil {
				return incomingObj, err
			}
		}

		if params.regType == v1.RegistrationModeOnline {
			regCodeSecret, err := h.regCodeFromSecretEntrypoint(params)
			if err != nil {
				return incomingObj, err
			}

			if _, err := h.secretRepo.CreateOrUpdateSecret(regCodeSecret); err != nil {
				return incomingObj, err
			}
		}

		// construct associated registration CRs
		registration, err := h.registrationFromSecretEntrypoint(params)
		if err != nil {
			return incomingObj, fmt.Errorf("failed to create registration from secret %s/%s: %w", incomingObj.Namespace, incomingObj.Name, err)
		}

		if createOrUpdateErr := h.createOrUpdateRegistration(registration); createOrUpdateErr != nil {
			h.log.Errorf("failed to create or update registration %s: %v", registration.Name, createOrUpdateErr)
			return incomingObj, fmt.Errorf("failed to create or update registration %s: %w", registration.Name, createOrUpdateErr)
		}
	}

	return incomingObj, nil
}

func (h *handler) cleanupRegistrationByHash(cleanupRequest hashCleanupRequest) error {
	var regs []*v1.Registration
	var err error
	if cleanupRequest.hashType == ContentHash {
		regs, err = h.registrationCache.GetByIndex(IndexRegistrationsBySccHash, cleanupRequest.hash)
	} else {
		regs, err = h.registrationCache.GetByIndex(IndexRegistrationsByNameHash, cleanupRequest.hash)
	}

	h.log.Infof("found %d matching registrations to clean up the %s hash", len(regs), cleanupRequest.hashType)
	if err != nil {
		return err
	}

	for _, reg := range regs {
		if !slices.Contains(reg.Finalizers, consts.FinalizerSccRegistration) {
			continue
		}

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
			return fmt.Errorf("failed to delete registration %s: %w", reg.Name, deleteErr)
		}
	}
	return nil
}

func (h *handler) cleanupRelatedSecretsByHash(contentHash string) error {
	secrets, err := h.secretCache.GetByIndex(IndexSecretsBySccHash, contentHash)
	h.log.Infof("found %d matching related secrets to clean up; content hash of %s", len(secrets), contentHash)
	if err != nil {
		return err
	}

	// It should never be in there, but just in case don't act on the entrypoint
	secrets = slices.Collect(func(yield func(secret *corev1.Secret) bool) {
		for _, secret := range secrets {
			if secret.Name != consts.ResourceSCCEntrypointSecretName {
				if !yield(secret) {
					return
				}
			}
		}
	})

	for _, secret := range secrets {
		if common.SecretHasCredentialsFinalizer(secret) ||
			common.SecretHasRegCodeFinalizer(secret) {

			var updateErr error
			secretUpdated := secret.DeepCopy()
			secretUpdated, _ = common.SecretRemoveCredentialsFinalizer(secretUpdated)
			secretUpdated, _ = common.SecretRemoveRegCodeFinalizer(secretUpdated)
			secretUpdated, updateErr = h.secretRepo.RetryingPatchUpdate(secret, secretUpdated)
			if updateErr != nil {
				h.log.Errorf("failed to update secret %s/%s: %v", secret.Namespace, secret.Name, updateErr)
				return updateErr
			}
		}

		deleteErr := h.secrets.Delete(secret.Namespace, secret.Name, &metav1.DeleteOptions{})
		if apierrors.IsNotFound(deleteErr) {
			h.log.Debugf("Related Secret %s/%s already deleted", secret.Namespace, secret.Name)
			continue
		}
		if deleteErr != nil {
			return fmt.Errorf("failed to delete secret %s/%s: %w", secret.Namespace, secret.Name, deleteErr)
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

		// TODO: (alex) needs some thought about how we actually map entrypoint secret cleanup
		// here based on the control flow changes in OnChange
		if err := h.cleanupRegistrationByHash(hashCleanupRequest{
			hash,
			ContentHash,
		}); err != nil {
			h.log.Errorf("failed to cleanup registrations for hash %s: %v", hash, err)
			return nil, err
		}
		if cleanUpErr := h.cleanupRelatedSecretsByHash(hash); cleanUpErr != nil {
			h.log.Errorf("failed to cleanup registrations for hash %s: %v", hash, cleanUpErr)
			return incomingObj, cleanUpErr
		}

		return incomingObj, nil
	}

	if common.SecretHasCredentialsFinalizer(incomingObj) ||
		common.SecretHasRegCodeFinalizer(incomingObj) {
		refs := incomingObj.GetOwnerReferences()
		danglingRefs := 0
		for _, ref := range refs {
			if ref.APIVersion == v1.SchemeGroupVersion.String() &&
				ref.Kind == "Registration" {
				reg, err := h.registrations.Get(ref.Name, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					continue
				} else {
					if reg.DeletionTimestamp == nil {
						danglingRefs++
					} else {
						// TODO(alex): verify this logic when you are back
						// When reg is marked to delete too - we may need to help clean it up
						regFinalizers := reg.GetFinalizers()
						if len(regFinalizers) > 0 && slices.Contains(regFinalizers, consts.FinalizerSccRegistration) {
							regUpdate := reg.DeepCopy()
							removeIndex := slices.Index(regFinalizers, consts.FinalizerSccRegistration)
							regUpdate.Finalizers = append(reg.Finalizers[:removeIndex], reg.Finalizers[removeIndex+1:]...)
							_, err = h.patchUpdateRegistration(reg, regUpdate)
							if err != nil {
								h.log.Errorf("failed to patch registration %s/%s: %v", reg.Namespace, reg.Name, err)
							}
						}
					}
				}
			}
		}
		if danglingRefs > 0 {
			h.log.Errorf("cannot remove SCC finalizer from secret %s/%s, dangling references to Registration found", incomingObj.Namespace, incomingObj.Name)
			return nil, fmt.Errorf("cannot remove SCC finalizer from secret %s/%s, dangling references to Registration found", incomingObj.Namespace, incomingObj.Name)
		}
		newSecret := incomingObj.DeepCopy()
		if common.SecretHasCredentialsFinalizer(newSecret) {
			newSecret, _ = common.SecretRemoveCredentialsFinalizer(newSecret)
		}
		if common.SecretHasRegCodeFinalizer(newSecret) {
			newSecret, _ = common.SecretRemoveRegCodeFinalizer(newSecret)
		}
		logrus.Info("Removing finalizer from secret", newSecret.Name, "in namespace", newSecret.Namespace)
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			_, err := h.secretRepo.PatchUpdate(incomingObj, newSecret)
			return err
		}); err != nil {
			h.log.Errorf("failed to remove SCC finalizer from secret %s/%s: %v", incomingObj.Namespace, incomingObj.Name, err)
			return nil, fmt.Errorf("failed to remove SCC finalizer from secret %s/%s: %w", incomingObj.Namespace, incomingObj.Name, err)
		}
	}

	return incomingObj, nil
}

func (h *handler) OnRegistrationChange(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	activiateMu.Lock()
	defer activiateMu.Unlock()
	if registrationObj == nil || registrationObj.DeletionTimestamp != nil {
		return nil, nil
	}

	if !systeminfo.IsServerUrlReady() {
		h.log.Info("Server URL not set")
		return registrationObj, errors.New("no server url found in the system info")
	}

	if common.RegistrationIsFailed(registrationObj) {
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
		if !registrationObj.HasCondition(v1.ResourceConditionProgressing) || v1.ResourceConditionProgressing.IsFalse(registrationObj) {
			progressingObj := registrationObj.DeepCopy()
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

			return registrationObj, nil
		}

		// Start of initial registration/announce of cluster
		regForAnnounce := registrationObj.DeepCopy()
		preparedForRegister, prepareErr := registrationHandler.PrepareForRegister(regForAnnounce)
		if prepareErr != nil {
			err := h.reconcileRegistration(registrationHandler, preparedForRegister, prepareErr, types.RegistrationPrepare)
			return registrationObj, err
		}

		var updateErr error
		if regForAnnounce, updateErr = h.registrations.UpdateStatus(preparedForRegister); updateErr != nil {
			return registrationObj, updateErr
		}

		announcedSystemId, registerErr := registrationHandler.Register(regForAnnounce)
		if registerErr != nil {
			err := h.reconcileRegistration(registrationHandler, preparedForRegister, registerErr, types.RegistrationMain)
			return registrationObj, err
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
			err := h.reconcileRegistration(registrationHandler, preparedForRegister, prepareError, types.RegistrationForActivation)
			return registrationObj, err
		}
		regForAnnounce.Status.RegistrationProcessedTS = &metav1.Time{
			Time: time.Now(),
		}

		_, registerUpdateErr := h.registrations.UpdateStatus(regForAnnounce)
		if registerUpdateErr != nil {
			return registrationObj, registerUpdateErr
		}

		return registrationObj, nil
	}

	if registrationHandler.NeedsActivation(registrationObj) {
		if !registrationHandler.ReadyForActivation(registrationObj) {
			h.log.Debugf("registration needs to be activated, but not yet ready; %v", registrationObj)
			return registrationObj, nil
		}
		activationErr := registrationHandler.Activate(registrationObj)
		// reconcile error state - must be able to handle Auth errors (or other SCC sourced errors)
		if activationErr != nil {
			err := h.reconcileActivation(registrationHandler, registrationObj, activationErr, types.ActivationMain)
			return registrationObj, err
		}

		activatedUpdateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var retryErr, updateErr error
			registrationObj, retryErr = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
			if retryErr != nil {
				return retryErr
			}

			activated := registrationObj.DeepCopy()
			activated = common.PrepareSuccessfulActivation(activated)
			prepared, err := registrationHandler.PrepareActivatedForKeepalive(activated)
			if err != nil {
				err := h.reconcileActivation(registrationHandler, registrationObj, activationErr, types.ActivationPrepForKeepalive)
				return err
			}
			_, updateErr = h.registrations.UpdateStatus(prepared)
			return updateErr
		})
		if activatedUpdateErr != nil {
			return registrationObj, activatedUpdateErr
		}

		return registrationObj, nil
	}

	// Handle what to do when CheckNow is used...
	if common.RegistrationNeedsSyncNow(registrationObj) {
		if registrationObj.Spec.Mode == v1.RegistrationModeOffline {
			updated := registrationObj.DeepCopy()
			updated.Spec = *registrationObj.Spec.WithoutSyncNow()

			offlineHandler := registrationHandler.(sccOfflineMode)
			refreshErr := offlineHandler.RefreshOfflineRequestSecret()
			_, updateErr := h.registrations.Update(updated)

			return registrationObj, errors.Join(refreshErr, updateErr)
		} else {
			// Todo: online/offline  handler should have a sync now
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
			return registrationObj, err
		}
	}

	keepaliveErr := registrationHandler.Keepalive(registrationObj)
	if keepaliveErr != nil {
		reconcileErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			curReg, getErr := h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
			if getErr != nil {
				return getErr
			}

			prepareObj := curReg.DeepCopy()
			prepareObj = registrationHandler.ReconcileKeepaliveError(prepareObj, keepaliveErr)

			_, reconcileUpdateErr := h.registrations.Update(prepareObj)
			return reconcileUpdateErr
		})

		err := fmt.Errorf("keepalive failed: %w", keepaliveErr)
		if reconcileErr != nil {
			err = fmt.Errorf("keepalive failed with additional errors: %w, %w", keepaliveErr, reconcileErr)
		}

		return registrationObj, err
	}

	keepaliveUpdateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var retryErr, updateErr error
		registrationObj, retryErr = h.registrations.Get(registrationObj.Name, metav1.GetOptions{})
		if retryErr != nil {
			return retryErr
		}

		keepalive := registrationObj.DeepCopy()
		keepalive = common.PrepareSuccessfulActivation(keepalive)
		v1.RegistrationConditionKeepalive.True(keepalive)
		prepared, err := registrationHandler.PrepareKeepaliveSucceeded(keepalive)
		if err != nil {
			return err
		}
		_, updateErr = h.registrations.UpdateStatus(prepared)
		return updateErr
	})
	if keepaliveUpdateErr != nil {
		return registrationObj, keepaliveUpdateErr
	}

	return registrationObj, nil
}

func (h *handler) OnRegistrationRemove(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, nil
	}

	regHandler := h.prepareHandler(registrationObj)
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

func scopedOnChange[T generic.RuntimeMetaObject](ctx context.Context, name, namespace string, c generic.ControllerMeta, sync generic.ObjectHandler[T]) {
	condition := namespaceScopedCondition(namespace)
	onChangeHandler := generic.FromObjectHandlerToHandler(sync)
	c.AddGenericHandler(ctx, name, func(key string, obj runtime.Object) (runtime.Object, error) {
		if condition(obj) {
			return onChangeHandler(key, obj)
		}
		return obj, nil
	})
}

// TODO(wrangler/v4): revert to use OnRemove when it supports options (https://github.com/rancher/wrangler/pull/472).
func scopedOnRemove[T generic.RuntimeMetaObject](ctx context.Context, name, namespace string, c generic.ControllerMeta, sync generic.ObjectHandler[T]) {
	condition := namespaceScopedCondition(namespace)
	onRemoveHandler := generic.NewRemoveHandler(name, c.Updater(), generic.FromObjectHandlerToHandler(sync))
	c.AddGenericHandler(ctx, name, func(key string, obj runtime.Object) (runtime.Object, error) {
		if condition(obj) {
			return onRemoveHandler(key, obj)
		}
		return obj, nil
	})
}

func namespaceScopedCondition(namespace string) func(obj runtime.Object) bool {
	return func(obj runtime.Object) bool { return inExpectedNamespace(obj, namespace) }
}

func inExpectedNamespace(obj runtime.Object, namespace string) bool {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false
	}

	return metadata.GetNamespace() == namespace
}
