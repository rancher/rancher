package registration

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/suseconnect/credentials"
	"github.com/rancher/rancher/pkg/scc/util"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

type Handler struct {
	ctx            context.Context
	registrations  registrationControllers.RegistrationController
	secrets        v1core.SecretController
	sccCredentials *credentials.CredentialSecretsAdapter
	systemInfo     *util.RancherSystemInfo
}

func New(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	sccCredentials *credentials.CredentialSecretsAdapter,
	systemInfo *util.RancherSystemInfo,
) *Handler {
	controller := &Handler{
		ctx:            ctx,
		registrations:  registrations,
		secrets:        secrets,
		sccCredentials: sccCredentials,
		systemInfo:     systemInfo,
	}

	return controller
}

func (h *Handler) Call(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	logrus.Infof("[scc.registration-controller]: Received registration %q", name)
	logrus.Info("[scc.registration-controller]: Registration ", registrationObj)

	// TODO: implement expiration - gist: Registration shouldn't repeat to infinity when issues.
	// Ideally we would eventually timeout a Registration after it fails X times or for X minutes
	if registrationObj.Status.RegistrationProcessedTS != nil {
		logrus.Info("[scc.registration-controller]: Registration already processed")
		return registrationObj, nil
	}

	var err error
	if registrationObj.Spec.Mode == v1.Online {
		onlineHandlerObj := &onlineHandler{
			rootHandler: h,
		}
		registrationObj, err = onlineHandlerObj.Run(registrationObj)
		if err != nil {
			return h.setReconcilingCondition(registrationObj, err)
		}
	} else {
		offlineHandlerObj := &offlineHandler{
			rootHandler: h,
		}
		registrationObj, err = offlineHandlerObj.Run(registrationObj)
		if err != nil {
			return h.setReconcilingCondition(registrationObj, err)
		}
	}

	if registrationObj.Status.RegistrationProcessedTS != nil {
		h.registrations.Enqueue(registrationObj.Name)
	}

	return registrationObj, nil
}

func (h *Handler) setReconcilingCondition(request *v1.Registration, originalErr error) (*v1.Registration, error) {
	logrus.Info("[scc.registration-controller]: set reconciling condition")
	logrus.Error(originalErr)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		updBackup, err := h.registrations.Get(request.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		updBackup = updBackup.DeepCopy()
		v1.ResourceConditionFailure.SetStatusBool(updBackup, true)
		v1.ResourceConditionFailure.SetError(updBackup, "", originalErr)
		v1.ResourceConditionReady.Message(updBackup, "Retrying")
		v1.ResourceConditionProgressing.False(updBackup)

		_, err = h.registrations.UpdateStatus(updBackup)
		return err
	})
	if err != nil {
		return request, errors.New(originalErr.Error() + err.Error())
	}

	return request, originalErr
}
