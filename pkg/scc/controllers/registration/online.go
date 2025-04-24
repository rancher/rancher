package registration

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type onlineHandler struct {
	rootHandler *handler
}

func (oh *onlineHandler) Run(registrationObj *v1.Registration) (*v1.Registration, error) {
	// This is an extra verification borne of paranoia - technically the controller won't start till this is ready.
	// However, people can do silly things so if someone unsets the server URL this will block that.
	if !oh.rootHandler.isServerUrlReady() {
		return registrationObj, errors.New("cannot process registration if `server-url` is not configured")
	}

	if v1.ResourceConditionDone.IsTrue(registrationObj) && v1.RegistrationConditionAnnounced.IsTrue(registrationObj) {
		logrus.Debugf("[scc.registration-controller]: registration already complete, nothing to process for %s", registrationObj.Name)
		return registrationObj, nil
	}

	logrus.Debug("[scc.registration-controller]: online mode registration starting")
	credErr := oh.rootHandler.sccCredentials.Refresh() // We must always refresh the sccCredentials - this ensures they are current from the secrets
	if credErr != nil {
		return registrationObj, fmt.Errorf("cannot refresh credentials: %w", credErr)
	}

	v1.ResourceConditionProgressing.SetStatusBool(registrationObj, true)
	registrationObj, err := oh.rootHandler.registrations.UpdateStatus(registrationObj)
	if err != nil {
		return registrationObj, err
	}

	// Set to global default, or user configured value from the Registration resource
	regCodeSecretRef := &corev1.SecretReference{
		Namespace: "cattle-system",
		Name:      util.RegCodeSecretName,
	}
	if registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef != nil {
		regObjRegCodeSecretRef := registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef
		if regObjRegCodeSecretRef.Name != "" && regObjRegCodeSecretRef.Namespace != "" {
			regCodeSecretRef = registrationObj.Spec.RegistrationRequest.RegistrationCodeSecretRef
		} else {
			logrus.Warn("[scc.registration-controller]: registration code secret reference was set but cannot be used")
		}
	}

	// Initiate connection to SCC & verify reg code is for Rancher
	sccConnection := suseconnect.DefaultRancherConnection(oh.rootHandler.sccCredentials.SccCredentials(), oh.rootHandler.systemInfo)

	// Announce this Rancher cluster to SCC
	registrationObj, err = oh.announceSystem(registrationObj, &sccConnection, regCodeSecretRef)
	if err != nil {
		return registrationObj, err
	}

	// Prepare the Registration for Activation phase next
	completeObj := registrationObj.DeepCopy()
	completeObj.Status.RegistrationStatus.RequestProcessedTS = time.Now().UTC().Format(time.RFC3339)
	completeObj.Status.Conditions = make([]genericcondition.GenericCondition, 0)
	v1.ResourceConditionFailure.SetStatusBool(completeObj, false)
	v1.ResourceConditionReady.SetStatusBool(completeObj, true)
	completeObj, finalUpdateErr := oh.rootHandler.registrations.UpdateStatus(completeObj)
	if finalUpdateErr != nil {
		return registrationObj, finalUpdateErr
	}

	return completeObj, nil
}

func (oh *onlineHandler) announceSystem(registrationObj *v1.Registration, sccConnection *suseconnect.SccWrapper, regCodeSecretRef *corev1.SecretReference) (*v1.Registration, error) {
	// Fetch the SCC registration code; for 80% of users this should be a real code
	// The other cases are either:
	//	a. an error and should have had a code, OR
	//	b. BAYG/RMT/etc based Registration and will not use a code
	registrationCode := suseconnect.FetchSccRegistrationCodeFrom(oh.rootHandler.secrets, regCodeSecretRef)

	id, regErr := sccConnection.RegisterOrKeepAlive(registrationCode)
	if regErr != nil {
		// TODO, do we error different based on ID type?
		return registrationObj, regErr
	}

	if id == suseconnect.KeepAliveRegistrationSystemId {
		// TODO something to update status from keepalive
		return registrationObj, nil
	}

	sccSystemUrl := fmt.Sprintf("https://scc.suse.com/system/%d", id)
	logrus.Debugf("[scc.registration-controller]: system announced, check %s", sccSystemUrl)

	newRegObj := registrationObj.DeepCopy()
	v1.RegistrationConditionSccUrlReady.SetStatusBool(newRegObj, false) // This must be false until successful activation too.
	v1.RegistrationConditionSccUrlReady.SetMessageIfBlank(newRegObj, fmt.Sprintf("system announced, check %s", sccSystemUrl))

	newRegObj.Status.RegistrationStatus.SCCSystemId = int(id)
	newRegObj.Status.SystemCredentialsSecretRef = &corev1.SecretReference{
		Namespace: credentials.Namespace,
		Name:      credentials.SecretName,
	}

	var updateErr error
	newRegObj, updateErr = oh.rootHandler.registrations.UpdateStatus(newRegObj)
	if updateErr != nil {
		return registrationObj, updateErr
	}

	return newRegObj, nil
}
