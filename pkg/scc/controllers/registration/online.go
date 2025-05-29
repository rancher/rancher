package registration

import (
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"time"
)

type onlineHandler struct {
	rootHandler *Handler
}

func (oh *onlineHandler) Run(registrationObj *v1.Registration) (*v1.Registration, error) {
	if v1.ResourceConditionDone.IsTrue(registrationObj) ||
		v1.RegistrationConditionAnnounced.IsTrue(registrationObj) {
		logrus.Debugf("[scc.registration-controller]: registration already complete, nothing to process for %s", registrationObj.Name)
		return registrationObj, nil
	}

	logrus.Debug("[scc.registration-controller]: online mode registration starting")
	credErr := oh.rootHandler.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credErr != nil {
		return registrationObj, fmt.Errorf("cannot refresh credentials: %w", credErr)
	}

	progressingObj := registrationObj.DeepCopy()
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		v1.ResourceConditionProgressing.True(progressingObj)
		progressingObj, err = oh.rootHandler.registrations.UpdateStatus(progressingObj)
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
			logrus.Warn("[scc.registration-controller]: registration code secret reference was set but cannot be used")
		}
	}

	// Initiate connection to SCC & verify reg code is for Rancher
	sccConnection := suseconnect.DefaultRancherConnection(oh.rootHandler.sccCredentials.SccCredentials(), oh.rootHandler.systemInfoExporter)

	// Announce this Rancher cluster to SCC
	announcedReg, announceErr := oh.announceSystem(progressingObj, &sccConnection, regCodeSecretRef)
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
		updatingObj, err = oh.rootHandler.registrations.UpdateStatus(updatingObj)

		return err
	})
	if updateErr != nil {
		return registrationObj, updateErr
	}

	return updatingObj, nil
}

func (oh *onlineHandler) announceSystem(registrationObj *v1.Registration, sccConnection *suseconnect.SccWrapper, regCodeSecretRef *corev1.SecretReference) (*v1.Registration, error) {
	// Fetch the SCC registration code; for 80% of users this should be a real code
	// The other cases are either:
	//	a. an error and should have had a code, OR
	//	b. BAYG/RMT/etc based Registration and will not use a code
	registrationCode := suseconnect.FetchSccRegistrationCodeFrom(oh.rootHandler.secrets, regCodeSecretRef)

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
	logrus.Debugf("[scc.registration-controller]: system announced, check %s", sccSystemUrl)

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

		newRegObj, err = oh.rootHandler.registrations.UpdateStatus(newRegObj)
		return err
	})
	if updateErr != nil {
		return registrationObj, updateErr
	}

	return newRegObj, nil
}
