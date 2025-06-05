package controllers

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"github.com/rancher/wrangler/v3/pkg/apply"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type sccOfflineMode struct {
	apply              apply.Apply
	log                log.StructuredLogger
	registrations      registrationControllers.RegistrationController
	systemInfoExporter *systeminfo.InfoExporter
	secrets            v1core.SecretController
}

func (s sccOfflineMode) NeedsRegistration(registrationObj *v1.Registration) bool {
	return registrationObj.Status.RegistrationProcessedTS.IsZero() ||
		!registrationObj.HasCondition(v1.RegistrationConditionOfflineRequestReady) ||
		v1.RegistrationConditionOfflineRequestReady.IsFalse(registrationObj)
}

func (s sccOfflineMode) RegisterSystem(registrationObj *v1.Registration) (*v1.Registration, error) {
	if v1.ResourceConditionDone.IsTrue(registrationObj) ||
		v1.RegistrationConditionAnnounced.IsTrue(registrationObj) {
		logrus.Debugf("[scc.registration-controller]: registration already complete, nothing to process for %s", registrationObj.Name)
		return registrationObj, nil
	}

	// TODO: this generation and secret maybe should be updated regularly like Online mode phone home?
	generatedRegistrationRequest, err := s.systemInfoExporter.PreparedForSCCOffline()
	if err != nil {
		return nil, err
	}

	createdSecret := suseconnect.CreateSccOfflineRegistrationRequestSecret(generatedRegistrationRequest)
	applyErr := s.apply.ApplyObjects(createdSecret)
	if applyErr != nil {
		return nil, applyErr
	}

	updatingObj := registrationObj.DeepCopy()
	updatingObj.Status.OfflineRegistrationRequest = &corev1.SecretReference{
		Name:      createdSecret.Name,
		Namespace: createdSecret.Namespace,
	}
	updatingObj.Status.RegistrationProcessedTS = &metav1.Time{
		Time: time.Now(),
	}
	v1.RegistrationConditionOfflineRequestReady.True(updatingObj)
	return s.registrations.UpdateStatus(updatingObj)
}

func (s sccOfflineMode) NeedsActivation(registrationObj *v1.Registration) bool {
	return !registrationObj.Status.ActivationStatus.Activated ||
		registrationObj.Status.ActivationStatus.LastValidatedTS == nil
}

func (s sccOfflineMode) Activate(registrationObj *v1.Registration) error {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) Keepalive(registrationObj *v1.Registration) (*v1.Registration, error) {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) Deregister() error {
	//TODO implement me
	panic("implement me")
}

var _ SCCHandler = &sccOfflineMode{}
