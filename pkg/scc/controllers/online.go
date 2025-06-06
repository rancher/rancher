package controllers

import (
	"errors"
	"fmt"
	"github.com/SUSE/connect-ng/pkg/connection"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/scc/util/log"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

type sccOnlineMode struct {
	log                log.StructuredLogger
	sccCredentials     *credentials.CredentialSecretsAdapter
	systemInfoExporter *systeminfo.InfoExporter
	secrets            v1core.SecretController
}

func (s sccOnlineMode) NeedsRegistration(registrationObj *v1.Registration) bool {
	return registrationObj.Status.RegistrationProcessedTS.IsZero() ||
		!registrationObj.HasCondition(v1.RegistrationConditionSccUrlReady) ||
		!registrationObj.HasCondition(v1.RegistrationConditionAnnounced)
}

func (s sccOnlineMode) RegisterSystem(registrationObj *v1.Registration) (suseconnect.RegistrationSystemId, error) {
	credRefreshErr := s.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credRefreshErr != nil {
		return suseconnect.EmptyRegistrationSystemId, fmt.Errorf("cannot refresh credentials: %w", credRefreshErr)
	}

	// Fetch the SCC registration code; for 80% of users this should be a real code
	// The other cases are either:
	//	a. an error and should have had a code, OR
	//	b. BAYG/RMT/etc based Registration and will not use a code
	registrationCode := s.fetchRegCode(registrationObj)

	// Initiate connection to SCC & verify reg code is for Rancher
	sccConnection := suseconnect.DefaultRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter)

	// RegisterSystem this Rancher cluster to SCC
	id, regErr := sccConnection.RegisterOrKeepAlive(registrationCode)
	if regErr != nil {
		// TODO(scc) do we error different based on ID type?
		return id, regErr
	}

	return id, nil
}

func (s sccOnlineMode) ReconcileRegisterSystemError(registration *v1.Registration, registerErr error) *v1.Registration {
	return registration
}

func (s sccOnlineMode) PrepareRegisteredSystem(registration *v1.Registration) *v1.Registration {
	sccSystemUrl := fmt.Sprintf("https://scc.suse.com/systems/%d", registration.Status.SCCSystemId)
	s.log.Debugf("system announced, check %s", sccSystemUrl)

	prepared := registration.DeepCopy()
	prepared.Status.ActivationStatus.SystemUrl = sccSystemUrl
	v1.RegistrationConditionSccUrlReady.SetStatusBool(prepared, false) // This must be false until successful activation too.
	v1.RegistrationConditionSccUrlReady.SetMessageIfBlank(prepared, fmt.Sprintf("system announced, check %s", sccSystemUrl))
	v1.RegistrationConditionAnnounced.SetStatusBool(prepared, true)
	v1.ResourceConditionFailure.SetStatusBool(prepared, false)
	v1.ResourceConditionReady.SetStatusBool(prepared, true)

	prepared.Status.SystemCredentialsSecretRef = &corev1.SecretReference{
		Namespace: credentials.Namespace,
		Name:      credentials.SecretName,
	}

	return prepared
}

func (s sccOnlineMode) fetchRegCode(registrationObj *v1.Registration) string {
	// Set to global default, or user configured value from the Registration resource
	regCodeSecretRef := &corev1.SecretReference{
		Namespace: "cattle-system",
		Name:      util.RegCodeSecretName,
	}
	if registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef != nil {
		regObjRegCodeSecretRef := registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef
		if regObjRegCodeSecretRef.Name != "" && regObjRegCodeSecretRef.Namespace != "" {
			regCodeSecretRef = regObjRegCodeSecretRef
		} else {
			s.log.Warn("registration code secret reference was set but cannot be used")
		}
	}

	return suseconnect.FetchSccRegistrationCodeFrom(s.secrets, regCodeSecretRef)
}

func (s sccOnlineMode) NeedsActivation(registrationObj *v1.Registration) bool {
	return !registrationObj.Status.ActivationStatus.Activated ||
		registrationObj.Status.ActivationStatus.LastValidatedTS == nil
}

func (s sccOnlineMode) Activate(registrationObj *v1.Registration) error {
	if v1.RegistrationConditionAnnounced.IsFalse(registrationObj) {
		// reconcile to set failed status if not set
		return errors.New("cannot activate system that hasn't been announced to SCC")
	}

	s.log.Debugf("received registration ready for activations %q", registrationObj.Name)
	s.log.Debug("registration ", registrationObj)

	credRefreshErr := s.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credRefreshErr != nil {
		return fmt.Errorf("cannot refresh credentials: %w", credRefreshErr)
	}

	registrationCode := s.fetchRegCode(registrationObj)
	sccConnection := suseconnect.DefaultRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter)

	identifier, version, arch := s.systemInfoExporter.Provider().GetProductIdentifier()
	metaData, product, err := sccConnection.Activate(identifier, version, arch, registrationCode)
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

	s.log.Info("Successfully registered activation")

	return nil
}

// ReconcileActivateError will first verify if an error is recoverable and then reconcile the error as needed
func (s sccOnlineMode) ReconcileActivateError(registration *v1.Registration, activationErr error) *v1.Registration {
	preparedForReconcile := registration.DeepCopy()

	var myErr connection.ApiError
	if errors.As(activationErr, &myErr) {
		httpCode := myErr.Code
		// if 401 code assume system creds are outdated and invalidate the registration
		if httpCode == http.StatusUnauthorized {
			nowTime := metav1.Now()
			// Cannot recover from this error so must set failure
			preparedForReconcile.Status.ActivationStatus.Activated = false
			preparedForReconcile.Status.ActivationStatus.LastValidatedTS = &nowTime
			v1.ResourceConditionFailure.True(preparedForReconcile)
			v1.ResourceConditionFailure.Message(preparedForReconcile, "Non-recoverable error encountered; to reregister Rancher create a new registration.")
			v1.ResourceConditionProgressing.False(preparedForReconcile)
			v1.ResourceConditionReady.False(preparedForReconcile)
			v1.RegistrationConditionActivated.SetError(preparedForReconcile, "Error: SCC sync returned Unauthorized (401) status", activationErr)

			return preparedForReconcile
		}
	}

	// Return the unmodified version
	return registration
}

func (s sccOnlineMode) Keepalive(registrationObj *v1.Registration) error {
	credRefreshErr := s.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credRefreshErr != nil {
		return fmt.Errorf("cannot refresh credentials: %w", credRefreshErr)
	}

	regCode := suseconnect.FetchSccRegistrationCodeFrom(s.secrets, registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef)
	sccConnection := suseconnect.DefaultRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter)

	identifier, version, arch := s.systemInfoExporter.Provider().GetProductIdentifier()
	metaData, product, err := sccConnection.Activate(identifier, version, arch, regCode)
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

func (s sccOnlineMode) ReconcileKeepaliveError(registration *v1.Registration, err error) *v1.Registration {
	//TODO implement me
	panic("implement me")
}

func (s sccOnlineMode) Deregister() error {
	_ = s.sccCredentials.Refresh()
	sccConnection := suseconnect.DefaultRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter)
	err := sccConnection.Deregister()
	if err != nil {
		return err
	}

	// Delete SCC credentials after successful Deregister
	credErr := s.sccCredentials.Remove()
	if credErr != nil {
		return credErr
	}

	return nil
}

var _ SCCHandler = &sccOnlineMode{}
