package controllers

import (
	"fmt"
	"github.com/pkg/errors"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/scc/util/log"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"time"
)

type sccOnlineMode struct {
	log                log.StructuredLogger
	registrations      registrationControllers.RegistrationController
	sccCredentials     *credentials.CredentialSecretsAdapter
	systemInfoExporter *systeminfo.InfoExporter
	secrets            v1core.SecretController
}

func (s sccOnlineMode) NeedsRegistration(obj *v1.Registration) bool {
	return obj.Status.RegistrationProcessedTS.IsZero() ||
		!obj.HasCondition(v1.RegistrationConditionSccUrlReady) ||
		!obj.HasCondition(v1.RegistrationConditionAnnounced)
}

func (s sccOnlineMode) RegisterSystem(registrationObj *v1.Registration) (*v1.Registration, error) {
	if v1.ResourceConditionDone.IsTrue(registrationObj) ||
		v1.RegistrationConditionAnnounced.IsTrue(registrationObj) {
		s.log.Debugf("registration already complete, nothing to process for %s", registrationObj.Name)
		return registrationObj, nil
	}

	credRefreshErr := s.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credRefreshErr != nil {
		return registrationObj, fmt.Errorf("cannot refresh credentials: %w", credRefreshErr)
	}

	progressingObj := registrationObj.DeepCopy()
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		v1.ResourceConditionProgressing.True(progressingObj)
		progressingObj, err = s.registrations.UpdateStatus(progressingObj)
		return err
	})
	if updateErr != nil {
		return registrationObj, updateErr
	}

	// Set to global default, or user configured value from the Registration resource
	regCodeSecretRef := &corev1.SecretReference{
		Namespace: "cattle-system",
		Name:      util.RegCodeSecretName,
	}
	if progressingObj.Spec.RegistrationRequest.RegistrationCodeSecretRef != nil {
		regObjRegCodeSecretRef := progressingObj.Spec.RegistrationRequest.RegistrationCodeSecretRef
		if regObjRegCodeSecretRef.Name != "" && regObjRegCodeSecretRef.Namespace != "" {
			regCodeSecretRef = regObjRegCodeSecretRef
		} else {
			s.log.Warn("registration code secret reference was set but cannot be used")
		}
	}

	// Initiate connection to SCC & verify reg code is for Rancher
	sccConnection := suseconnect.DefaultRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter)

	// RegisterSystem this Rancher cluster to SCC
	announcedReg, announceErr := s.announceSystem(progressingObj, &sccConnection, regCodeSecretRef)
	if announceErr != nil {
		return progressingObj, announceErr
	}

	// Prepare the Registration for Activation phase next
	updatingObj := announcedReg.DeepCopy()
	updateErr = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error

		updatingObj.Status.RegistrationProcessedTS = &metav1.Time{
			Time: time.Now(),
		}
		v1.ResourceConditionFailure.SetStatusBool(updatingObj, false)
		v1.ResourceConditionReady.SetStatusBool(updatingObj, true)
		updatingObj, err = s.registrations.UpdateStatus(updatingObj)

		return err
	})
	if updateErr != nil {
		return registrationObj, updateErr
	}

	return updatingObj, nil
}

func (s sccOnlineMode) announceSystem(registrationObj *v1.Registration, sccConnection *suseconnect.SccWrapper, regCodeSecretRef *corev1.SecretReference) (*v1.Registration, error) {
	// Fetch the SCC registration code; for 80% of users this should be a real code
	// The other cases are either:
	//	a. an error and should have had a code, OR
	//	b. BAYG/RMT/etc based Registration and will not use a code
	registrationCode := suseconnect.FetchSccRegistrationCodeFrom(s.secrets, regCodeSecretRef)

	id, regErr := sccConnection.RegisterOrKeepAlive(registrationCode)
	if regErr != nil {
		// TODO(scc) do we error different based on ID type?
		return registrationObj, regErr
	}

	if id == suseconnect.KeepAliveRegistrationSystemId {
		// TODO(scc) something to update status from keepalive
		return registrationObj, nil
	}

	sccSystemUrl := fmt.Sprintf("https://scc.suse.com/systems/%d", id)
	s.log.Debugf("system announced, check %s", sccSystemUrl)

	newRegObj := registrationObj.DeepCopy()
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		v1.RegistrationConditionSccUrlReady.SetStatusBool(newRegObj, false) // This must be false until successful activation too.
		v1.RegistrationConditionSccUrlReady.SetMessageIfBlank(newRegObj, fmt.Sprintf("system announced, check %s", sccSystemUrl))
		v1.RegistrationConditionAnnounced.SetStatusBool(newRegObj, true)

		newRegObj.Status.SCCSystemId = int(id)
		newRegObj.Status.SystemCredentialsSecretRef = &corev1.SecretReference{
			Namespace: credentials.Namespace,
			Name:      credentials.SecretName,
		}
		newRegObj.Status.ActivationStatus.SystemUrl = sccSystemUrl

		newRegObj, err = s.registrations.UpdateStatus(newRegObj)
		return err
	})
	if updateErr != nil {
		return registrationObj, updateErr
	}

	return newRegObj, nil
}

func (s sccOnlineMode) NeedsActivation(registrationObj *v1.Registration) bool {
	return !registrationObj.Status.ActivationStatus.Activated ||
		registrationObj.Status.ActivationStatus.LastValidatedTS == nil
}

func (s sccOnlineMode) Activate(registrationObj *v1.Registration) (*v1.Registration, error) {
	if v1.RegistrationConditionAnnounced.IsFalse(registrationObj) {
		// reconcile to set failed status if not set
		return registrationObj, errors.New("cannot activate system that hasn't been announced to SCC")
	}

	s.log.Infof("Received registration ready for activations %q", registrationObj.Name)
	s.log.Info("registration ", registrationObj)

	credRefreshErr := s.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credRefreshErr != nil {
		return registrationObj, fmt.Errorf("cannot refresh credentials: %w", credRefreshErr)
	}

	regCode := suseconnect.FetchSccRegistrationCodeFrom(s.secrets, registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef)
	sccConnection := suseconnect.DefaultRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter)

	identifier, version, arch := s.systemInfoExporter.Provider().GetProductIdentifier()
	metaData, product, err := sccConnection.Activate(identifier, version, arch, regCode)
	if err != nil {
		return registrationObj, err
	}
	s.log.Info(metaData)
	s.log.Info(product)

	// If no error, then system is still registered with valid activation status...
	keepAliveErr := sccConnection.KeepAlive()
	if keepAliveErr != nil {
		return registrationObj, keepAliveErr
	}

	s.log.Info("Successfully registered activation")
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		registrationObj, err = s.registrations.Get(registrationObj.Name, metav1.GetOptions{})
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
		registrationObj, err = s.registrations.UpdateStatus(updated)
		return err
	})

	return registrationObj, updateErr
}

func (s sccOnlineMode) Keepalive(registrationObj *v1.Registration) (*v1.Registration, error) {
	credRefreshErr := s.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credRefreshErr != nil {
		return registrationObj, fmt.Errorf("cannot refresh credentials: %w", credRefreshErr)
	}

	regCode := suseconnect.FetchSccRegistrationCodeFrom(s.secrets, registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef)
	sccConnection := suseconnect.DefaultRancherConnection(s.sccCredentials.SccCredentials(), s.systemInfoExporter)

	identifier, version, arch := s.systemInfoExporter.Provider().GetProductIdentifier()
	metaData, product, err := sccConnection.Activate(identifier, version, arch, regCode)
	if err != nil {
		return registrationObj, err
	}
	s.log.Info(metaData)
	s.log.Info(product)

	// If no error, then system is still registered with valid activation status...
	keepAliveErr := sccConnection.KeepAlive()
	if keepAliveErr != nil {
		return registrationObj, keepAliveErr
	}

	s.log.Info("Successfully checked in with SCC")
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		registrationObj, err = s.registrations.Get(registrationObj.Name, metav1.GetOptions{})
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
		registrationObj, err = s.registrations.UpdateStatus(updated)
		return err
	})

	return registrationObj, updateErr
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
