package controllers

import (
	"fmt"
	"github.com/SUSE/connect-ng/pkg/registration"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	offlineSecrets "github.com/rancher/rancher/pkg/scc/suseconnect/offline"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/systeminfo/offline"
	"github.com/rancher/rancher/pkg/scc/util/log"
)

type sccOfflineMode struct {
	registration       *v1.Registration
	log                log.StructuredLogger
	systemInfoExporter *systeminfo.InfoExporter
	offlineSecrets     *offlineSecrets.SecretManager
	systemNamespace    string
}

func (s sccOfflineMode) NeedsRegistration(registrationObj *v1.Registration) bool {
	return registrationObj.Spec.OfflineRegistrationCertificateSecretRef == nil &&
		(registrationObj.Status.RegistrationProcessedTS.IsZero() ||
			!registrationObj.HasCondition(v1.RegistrationConditionOfflineRequestReady) ||
			v1.RegistrationConditionOfflineRequestReady.IsFalse(registrationObj))
}

func (s sccOfflineMode) PrepareForRegister(registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj.Status.OfflineRegistrationRequest == nil {
		err := s.offlineSecrets.InitRequestSecret()
		if err != nil {
			return registrationObj, err
		}
		s.offlineSecrets.SetRegistrationOfflineRegistrationRequestSecretRef(registrationObj)
	}

	return registrationObj, nil
}

func (s sccOfflineMode) Register(registrationObj *v1.Registration) (suseconnect.RegistrationSystemId, error) {
	sccWrapper := suseconnect.OfflineRancherRegistration(s.systemInfoExporter)

	generatedOfflineRegistrationRequest, err := sccWrapper.PrepareOfflineRegistrationRequest()
	if err != nil {
		return suseconnect.EmptyRegistrationSystemId, err
	}
	updateErr := s.offlineSecrets.UpdateOfflineRequest(generatedOfflineRegistrationRequest)
	if updateErr != nil {
		return suseconnect.EmptyRegistrationSystemId, updateErr
	}

	return suseconnect.OfflineRegistrationSystemId, nil
}

func (s sccOfflineMode) ReconcileRegisterError(registrationObj *v1.Registration, registerErr error) *v1.Registration {
	return registrationObj
}

func (s sccOfflineMode) PrepareRegisteredForActivation(registrationObj *v1.Registration) (*v1.Registration, error) {

	v1.RegistrationConditionOfflineRequestReady.True(registrationObj)
	v1.RegistrationConditionOfflineCertificateReady.False(registrationObj)
	v1.RegistrationConditionOfflineCertificateReady.SetMessageIfBlank(registrationObj, "Awaiting registration certificate secret")

	return registrationObj, nil
}

func (s sccOfflineMode) NeedsActivation(registrationObj *v1.Registration) bool {
	return registrationObj.Status.OfflineRegistrationRequest != nil &&
		(!registrationObj.Status.ActivationStatus.Activated ||
			registrationObj.Status.ActivationStatus.LastValidatedTS.IsZero())
}

func (s sccOfflineMode) ReadyForActivation(registrationObj *v1.Registration) bool {
	return registrationObj.Status.OfflineRegistrationRequest != nil &&
		registrationObj.Spec.OfflineRegistrationCertificateSecretRef != nil
}

func (s sccOfflineMode) Activate(registrationObj *v1.Registration) error {
	// fetch secret contents (needs io.Reader)
	// registration.OfflineCertificateFrom()
	certReader, err := s.offlineSecrets.OfflineCertificateReader()
	if err != nil {
		return fmt.Errorf("activate failed, cannot get offline certificate reader: %w", err)
	}

	offlineCert, certErr := registration.OfflineCertificateFrom(certReader)
	if certErr != nil {
		return fmt.Errorf("activate failed, cannot prepare offline certificate: %w", certErr)
	}

	offlineCertValidator := offline.New(offlineCert, s.systemInfoExporter)

	return offlineCertValidator.ValidateCertificate()
}

func (s sccOfflineMode) PrepareActivatedForKeepalive(registrationObj *v1.Registration) (*v1.Registration, error) {
	v1.ActivationConditionOfflineDone.True(registrationObj)
	return registrationObj, nil
}

func (s sccOfflineMode) ReconcileActivateError(registrationObj *v1.Registration, activationErr error) *v1.Registration {
	//TODO implement me
	s.log.Error("implement me")
	return registrationObj
}

func (s sccOfflineMode) Keepalive(registrationObj *v1.Registration) error {
	s.log.Debugf("For now offline keepalive is an intentional noop")
	// TODO: eventually keepalive for offline should mimic `PrepareRegisteredForActivation` creation of ORR (to update metrics for next offline registration)
	return nil
}

func (s sccOfflineMode) PrepareKeepaliveSucceeded(registration *v1.Registration) (*v1.Registration, error) {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) ReconcileKeepaliveError(registration *v1.Registration, err error) *v1.Registration {
	s.log.Debugf("Because offline Keepalive is intentional noop, this shouldn't trigger")
	return registration
}

func (s sccOfflineMode) Deregister() error {
	// TODO implement me; for now this is no-op
	// TODO eventually this should clean up secrets downstream of this offline reg
	return nil
}

var _ SCCHandler = &sccOfflineMode{}
