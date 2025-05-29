package registration

import (
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
)

type offlineHandler struct {
	rootHandler *Handler
}

func (oh *offlineHandler) Run(registrationObj *v1.Registration) (*v1.Registration, error) {
	var updatedObj *v1.Registration
	var err error
	if registrationObj.Status.OfflineRegistrationRequest == nil {
		updatedObj, err = oh.prepareOfflineRegistrationRequest(registrationObj)
		if err != nil {
			return registrationObj, err
		}
	} else if registrationObj.Spec.OfflineRegistrationCertificateSecretRef != nil {
		updatedObj, err = oh.processOfflineRegistration(registrationObj)
		if err != nil {
			return registrationObj, err
		}
	}

	return updatedObj, nil
}

func (oh *offlineHandler) prepareOfflineRegistrationRequest(registrationObj *v1.Registration) (*v1.Registration, error) {
	logrus.Info("[scc.registration-controller]: offline mode create request")
	sccOfflineBlob, jsonErr := oh.rootHandler.systemInfoExporter.PreparedForSCCOffline()
	if jsonErr != nil {
		return registrationObj, jsonErr
	}
	offlineRegistrationSecret, err := suseconnect.StoreSccOfflineRegistration(oh.rootHandler.secrets, registrationObj, sccOfflineBlob)
	if err != nil {
		return registrationObj, err
	}

	updatedRequest := registrationObj.DeepCopy()
	updatedRequest.Status.OfflineRegistrationRequest = &corev1.SecretReference{
		Name:      offlineRegistrationSecret.Name,
		Namespace: offlineRegistrationSecret.Namespace,
	}

	// TODO(o&b): also set a status/condition to indicate RegistrationModeOffline is ready for user
	// The message could potentially even give command to fetch secret?

	updatedRequest, err = oh.rootHandler.registrations.UpdateStatus(updatedRequest)
	if err != nil {
		return registrationObj, err
	}

	return updatedRequest, nil
}

func (oh *offlineHandler) processOfflineRegistration(registrationObj *v1.Registration) (*v1.Registration, error) {
	// TODO(o&b) implement offline mechanism
	logrus.Info("[scc.registration-controller]: offline mode processing")
	return registrationObj, nil
}
