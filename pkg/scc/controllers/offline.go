package controllers

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util/log"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

type sccOfflineMode struct {
	log                log.StructuredLogger
	systemInfoExporter *systeminfo.InfoExporter
	secrets            v1core.SecretController
}

func (s sccOfflineMode) NeedsRegistration(registrationObj *v1.Registration) bool {
	return registrationObj.Status.RegistrationProcessedTS.IsZero() ||
		!registrationObj.HasCondition(v1.RegistrationConditionOfflineRequestReady) ||
		v1.RegistrationConditionOfflineRequestReady.IsFalse(registrationObj)
}

func (s sccOfflineMode) RegisterSystem(registrationObj *v1.Registration) (suseconnect.RegistrationSystemId, error) {
	// TODO: for offline it probably makes more sense to just return offline system ID const and do this prep in PrepareRegisteredSystem
	if v1.ResourceConditionDone.IsTrue(registrationObj) ||
		v1.RegistrationConditionAnnounced.IsTrue(registrationObj) {
		logrus.Debugf("[scc.registration-controller]: registration already complete, nothing to process for %s", registrationObj.Name)
		return suseconnect.EmptyRegistrationSystemId, nil
	}

	// TODO: this generation and secret maybe should be updated regularly like Online mode phone home?
	generatedRegistrationRequest, err := s.systemInfoExporter.PreparedForSCCOffline()
	if err != nil {
		return suseconnect.EmptyRegistrationSystemId, err
	}

	// TODO: actually save the secret via apply in the controller
	_ = suseconnect.CreateSccOfflineRegistrationRequestSecret(generatedRegistrationRequest)

	return suseconnect.OfflineRegistrationSystemId, nil
}

func (s sccOfflineMode) ReconcileRegisterSystemError(registration *v1.Registration, registerErr error) *v1.Registration {
	return registration
}

func (s sccOfflineMode) PrepareRegisteredSystem(registration *v1.Registration) *v1.Registration {
	return registration
}

func (s sccOfflineMode) NeedsActivation(registrationObj *v1.Registration) bool {
	return !registrationObj.Status.ActivationStatus.Activated ||
		registrationObj.Status.ActivationStatus.LastValidatedTS == nil
}

func (s sccOfflineMode) Activate(registrationObj *v1.Registration) error {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) ReconcileActivateError(registration *v1.Registration, activationErr error) *v1.Registration {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) Keepalive(registrationObj *v1.Registration) error {
	s.log.Debugf("For now offline keepalive is an intentional noop")
	return nil
}

func (s sccOfflineMode) ReconcileKeepaliveError(registration *v1.Registration, err error) *v1.Registration {
	s.log.Debugf("Because offline Keepalive is intentional noop, this sholdn't trigger")
	return registration
}

func (s sccOfflineMode) Deregister() error {
	//TODO implement me
	panic("implement me")
}

var _ SCCHandler = &sccOfflineMode{}
