package controllers

import (
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/controllers/common"
	"github.com/rancher/rancher/pkg/scc/types"
	"golang.org/x/sync/semaphore"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/SUSE/connect-ng/pkg/connection"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util/log"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	activiateMu sync.Mutex
	activeSem   *semaphore.Weighted = semaphore.NewWeighted(1)
)

type TryMutex struct {
	locked int32
}

func (m *TryMutex) TryLock() bool {
	return atomic.CompareAndSwapInt32(&m.locked, 0, 1)
}

func (m *TryMutex) Unlock() {
	atomic.StoreInt32(&m.locked, 0)
}

type sccOnlineMode struct {
	registration       *v1.Registration
	log                log.StructuredLogger
	sccCredentials     *credentials.CredentialSecretsAdapter
	systemInfoExporter *systeminfo.InfoExporter
	secrets            v1core.SecretController
	systemNamespace    string
}

func (s sccOnlineMode) NeedsRegistration(registrationObj *v1.Registration) bool {
	return registrationObj.Status.RegistrationProcessedTS.IsZero() ||
		!registrationObj.HasCondition(v1.RegistrationConditionSccUrlReady) ||
		!registrationObj.HasCondition(v1.RegistrationConditionAnnounced)
}

// PrepareForRegister creates the necessary SCC creds secret and secret reference
func (s sccOnlineMode) PrepareForRegister(registration *v1.Registration) (*v1.Registration, error) {
	if registration.Status.SystemCredentialsSecretRef == nil {
		err := s.sccCredentials.InitSecret()
		if err != nil {
			return registration, err
		}
		s.sccCredentials.SetRegistrationCredentialsSecretRef(registration)
	}

	return registration, nil
}

func (s sccOnlineMode) Register(registrationObj *v1.Registration) (suseconnect.RegistrationSystemId, error) {
	// We must always refresh the sccCredentials - this ensures they are current from the secrets
	credentialsErr := s.sccCredentials.Refresh()
	if credentialsErr != nil {
		return suseconnect.EmptyRegistrationSystemId, credentialsErr
	}

	// Fetch the SCC registration code; for 80% of users this should be a real code
	// The other cases are either:
	//	a. an error and should have had a code, OR
	//	b. BAYG/RMT/etc based Registration and will not use a code
	registrationCode := suseconnect.FetchSccRegistrationCodeFrom(s.secrets, registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef)

	regUrl := registrationObj.Spec.RegistrationRequest.RegistrationAPIUrl

	// Initiate connection to SCC & verify reg code is for Rancher
	sccConnection := suseconnect.OnlineRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter, *regUrl)

	// Register this Rancher cluster to SCC
	id, regErr := sccConnection.RegisterOrKeepAlive(registrationCode)
	if regErr != nil {
		// TODO(scc) do we error different based on ID type?
		return id, regErr
	}

	return id, nil
}

func (s sccOnlineMode) PrepareRegisteredForActivation(registration *v1.Registration) (*v1.Registration, error) {
	if registration.Status.SCCSystemId == nil {
		return registration, errors.New("SCC system ID cannot be empty when preparing registered system")
	}
	sccSystemUrl := fmt.Sprintf("https://scc.suse.com/systems/%d", *registration.Status.SCCSystemId)
	s.log.Debugf("system announced, check %s", sccSystemUrl)

	registration.Status.ActivationStatus.SystemUrl = &sccSystemUrl
	v1.RegistrationConditionSccUrlReady.SetStatusBool(registration, false) // This must be false until successful activation too.
	v1.RegistrationConditionSccUrlReady.SetMessageIfBlank(registration, fmt.Sprintf("system announced, check %s", sccSystemUrl))
	v1.RegistrationConditionAnnounced.SetStatusBool(registration, true)
	v1.ResourceConditionFailure.SetStatusBool(registration, false)
	v1.ResourceConditionReady.SetStatusBool(registration, true)

	return registration, nil
}

func isNonRecoverableHttpError(err error) bool {
	var sccApiError *connection.ApiError

	if errors.As(err, &sccApiError) {
		httpCode := sccApiError.Code

		// Client errors (except 429 Too Many Requests) are non-recoverable; a few server errors are also non-recoverable
		if (httpCode >= 400 && httpCode < 500 && httpCode != http.StatusTooManyRequests) ||
			httpCode == http.StatusNotImplemented ||
			httpCode == http.StatusHTTPVersionNotSupported ||
			httpCode == http.StatusNotExtended {
			return true
		}
	}
	return false
}

func getHttpErrorCode(err error) *int {
	var sccApiError *connection.ApiError

	if errors.As(err, &sccApiError) {
		httpCode := sccApiError.Code
		return &httpCode
	}
	return nil
}

type registrationReconcilerApplier func(regApplierIn *v1.Registration, httpCode *int) *v1.Registration

// reconcileNonRecoverableHttpError can help reconcile the registration state for any API/HTTP error related reasons
func (s sccOnlineMode) reconcileNonRecoverableHttpError(registrationIn *v1.Registration, registerErr error, additionalApplier registrationReconcilerApplier) *v1.Registration {
	httpCode := *getHttpErrorCode(registerErr)
	nowTime := metav1.Now()
	registrationIn.Status.RegistrationProcessedTS = &nowTime
	registrationIn.Status.ActivationStatus.LastValidatedTS = &nowTime
	v1.ResourceConditionFailure.True(registrationIn)
	v1.ResourceConditionFailure.Message(registrationIn, "Non-recoverable HTTP error encountered; to reregister Rancher, resolve connection issues then try again.")
	v1.ResourceConditionProgressing.False(registrationIn)
	v1.ResourceConditionReady.False(registrationIn)

	if additionalApplier != nil {
		return additionalApplier(registrationIn, &httpCode)
	}

	return registrationIn
}

func (s sccOnlineMode) ReconcileRegisterError(registrationObj *v1.Registration, registerErr error, phase types.RegistrationPhase) *v1.Registration {
	if isNonRecoverableHttpError(registerErr) {
		return s.reconcileNonRecoverableHttpError(
			registrationObj,
			registerErr,
			func(regApplierIn *v1.Registration, httpCode *int) *v1.Registration {
				preparedErrorReasonCondition := fmt.Sprintf("Error: SCC sync returned %s (%d) status", http.StatusText(*httpCode), httpCode)
				v1.RegistrationConditionAnnounced.SetError(regApplierIn, preparedErrorReasonCondition, registerErr)
				v1.RegistrationConditionSccUrlReady.False(regApplierIn)
				v1.RegistrationConditionActivated.False(regApplierIn)

				// Cannot recover from this error so must set failure
				regApplierIn.Status.ActivationStatus.Activated = false

				return regApplierIn
			},
		)
	}

	// TODO: any phase specific state updates

	return common.PrepareFailed(registrationObj, registerErr)
}

func (s sccOnlineMode) NeedsActivation(registrationObj *v1.Registration) bool {
	return !registrationObj.Status.ActivationStatus.Activated ||
		registrationObj.Status.ActivationStatus.LastValidatedTS.IsZero()
}

func (s sccOnlineMode) ReadyForActivation(registrationObj *v1.Registration) bool {
	return v1.RegistrationConditionAnnounced.IsTrue(registrationObj)
}

func (s sccOnlineMode) Activate(registrationObj *v1.Registration) error {
	s.log.Debugf("received registration ready for activations %q", registrationObj.Name)
	s.log.Debug("registration ", registrationObj)

	credentialsErr := s.sccCredentials.Refresh()
	if credentialsErr != nil {
		return fmt.Errorf("cannot load scc credentials: %w", credentialsErr)
	}

	registrationCode := suseconnect.FetchSccRegistrationCodeFrom(s.secrets, registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef)
	registrationUrl := registrationObj.Spec.RegistrationRequest.RegistrationAPIUrl
	sccConnection := suseconnect.OnlineRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter, *registrationUrl)

	metaData, product, err := sccConnection.Activate(registrationCode)
	if err != nil {
		return err
	}
	s.log.Info(metaData)
	s.log.Info(product)

	s.log.Info("Successfully registered activation")

	return nil
}

func (s sccOnlineMode) PrepareActivatedForKeepalive(registrationObj *v1.Registration) (*v1.Registration, error) {
	v1.RegistrationConditionSccUrlReady.True(registrationObj)

	credentialsErr := s.sccCredentials.Refresh()
	if credentialsErr != nil {
		return nil, fmt.Errorf("cannot load scc credentials: %w", credentialsErr)
	}
	regUrl := registrationObj.Spec.RegistrationRequest.RegistrationAPIUrl
	sccConnection := suseconnect.OnlineRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter, *regUrl)

	activations, err := sccConnection.ActivationStatus()
	if err != nil {
		return nil, err
	}
	if len(activations) == 0 {
		return nil, fmt.Errorf("no activations found for registration %q", registrationObj.Name)
	}
	// TODO: what if there are more than 1?
	firstActivation := activations[0]

	registrationObj.Status.RegistrationExpiresAt = &metav1.Time{Time: firstActivation.ExpiresAt}
	registrationObj.Status.RegisteredProduct = &firstActivation.Product.FriendlyName
	return registrationObj, nil
}

// ReconcileActivateError will first verify if an error is recoverable and then reconcile the error as needed
func (s sccOnlineMode) ReconcileActivateError(registration *v1.Registration, activationErr error, phase types.ActivationPhase) *v1.Registration {
	if isNonRecoverableHttpError(activationErr) {
		return s.reconcileNonRecoverableHttpError(
			registration,
			activationErr,
			func(regApplierIn *v1.Registration, httpCode *int) *v1.Registration {
				preparedErrorReasonCondition := fmt.Sprintf("Error: SCC sync returned %s (%d) status", http.StatusText(*httpCode), httpCode)
				v1.RegistrationConditionActivated.SetError(regApplierIn, preparedErrorReasonCondition, activationErr)

				// Cannot recover from this error so must set failure
				regApplierIn.Status.ActivationStatus.Activated = false

				return regApplierIn
			},
		)
	}

	// TODO other error reconcile when non-http error based

	// Return the unmodified version
	return registration
}

func (s sccOnlineMode) Keepalive(registrationObj *v1.Registration) error {
	credRefreshErr := s.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credRefreshErr != nil {
		return fmt.Errorf("cannot refresh credentials: %w", credRefreshErr)
	}

	regCode := suseconnect.FetchSccRegistrationCodeFrom(s.secrets, registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef)
	regUrl := registrationObj.Spec.RegistrationRequest.RegistrationAPIUrl
	sccConnection := suseconnect.OnlineRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter, *regUrl)

	metaData, product, err := sccConnection.Activate(regCode)
	if err != nil {
		return err
	}
	s.log.Info(metaData)
	s.log.Info(product)

	// If no error, then system is still registered with valid activation status...
	keepAliveErr := sccConnection.KeepAlive()
	if keepAliveErr != nil {
		return keepAliveErr
	}

	s.log.Info("Successfully checked in with SCC")

	return nil
}

func (s sccOnlineMode) PrepareKeepaliveSucceeded(registration *v1.Registration) (*v1.Registration, error) {
	// TODO take any post keepalive success steps
	s.log.Debug("preparing keepalive succeeded")
	return registration, nil
}

func (s sccOnlineMode) ReconcileKeepaliveError(registration *v1.Registration, keepaliveErr error) *v1.Registration {
	if isNonRecoverableHttpError(keepaliveErr) {
		return s.reconcileNonRecoverableHttpError(
			registration,
			keepaliveErr,
			func(regApplierIn *v1.Registration, httpCode *int) *v1.Registration {
				preparedErrorReasonCondition := fmt.Sprintf("Error: SCC sync returned %s (%d) status", http.StatusText(*httpCode), httpCode)
				v1.RegistrationConditionKeepalive.SetError(regApplierIn, preparedErrorReasonCondition, keepaliveErr)

				// Cannot recover from this error so must set failure
				regApplierIn.Status.ActivationStatus.Activated = false

				return regApplierIn
			},
		)
	}

	// TODO other error reconcile when non-http error based

	return registration
}

func (s sccOnlineMode) Deregister() error {
	_ = s.sccCredentials.Refresh()
	regUrl := s.registration.Spec.RegistrationRequest.RegistrationAPIUrl
	sccConnection := suseconnect.OnlineRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter, *regUrl)
	// TODO : this causes deletion to fail if the credentials are invalid. I think we
	// need to do a best effort check to see if it was ever registered before
	// we want to fail to delete if deregister fails, but the system is registered in SCC

	// Finalizers on the credential secret have helped this case, but it's still invalid if users edit the secret manually for some reason.
	err := sccConnection.Deregister()
	if err != nil {
		s.log.Warn("Deregister failure will be logged but not prevent cleanup")
		s.log.Errorf("Failed to deregister SCC registration: %v", err)
	}

	// Delete SCC credentials after successful Deregister
	credErr := s.sccCredentials.Remove()
	if credErr != nil {
		return credErr
	}

	// TODO refactor this to a cleanup
	regCodeSecretRef := s.registration.Spec.RegistrationRequest.RegistrationCodeSecretRef
	regCodeSecret, regCodeErr := s.secrets.Get(regCodeSecretRef.Namespace, regCodeSecretRef.Name, metav1.GetOptions{})
	if regCodeErr != nil {
		return regCodeErr
	}
	if slices.Contains(regCodeSecret.Finalizers, consts.FinalizerSccRegistrationCode) {
		index := slices.Index(regCodeSecret.Finalizers, consts.FinalizerSccRegistrationCode)
		regCodeSecret.Finalizers = slices.Delete(regCodeSecret.Finalizers, index, 1)
	}

	regCodeSecret, regCodeErr = s.secrets.Update(regCodeSecret)
	if regCodeErr != nil {
		return regCodeErr
	}

	regCodeErr = s.secrets.Delete(regCodeSecretRef.Namespace, regCodeSecretRef.Name, &metav1.DeleteOptions{})
	if regCodeErr != nil {
		return regCodeErr
	}

	return nil
}

var _ SCCHandler = &sccOnlineMode{}
