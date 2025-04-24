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

type handler struct {
	ctx            context.Context
	registrations  registrationControllers.RegistrationController
	secrets        v1core.SecretController
	sccCredentials *credentials.CredentialSecretsAdapter
	systemInfo     *util.RancherSystemInfo
}

func Register(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
	secrets v1core.SecretController,
	systemInfo *util.RancherSystemInfo,
) {
	controller := &handler{
		ctx:            ctx,
		registrations:  registrations,
		secrets:        secrets,
		sccCredentials: credentials.New(secrets),
		systemInfo:     systemInfo,
	}

	registrations.OnChange(ctx, "registrations", controller.OnRegistrationChange)
}

func (h *handler) OnRegistrationChange(name string, registrationObj *v1.Registration) (*v1.Registration, error) {
	if registrationObj == nil {
		return nil, nil
	}

	logrus.Infof("[scc.registration-controller]: Received registration %q", name)
	logrus.Info("[scc.registration-controller]: Registration ", registrationObj)
	// 1. Verify Registration is not already fulfilled or expired,
	// TODO: implement expiration - gist: Registration shouldn't repeat to infinity when issues.
	// Ideally we would eventually timeout a Registration after it fails X times or for X minutes
	if registrationObj.Status.RegistrationStatus.RequestProcessedTS != "" {
		logrus.Info("[scc.registration-controller]: Registration already processed")
		return registrationObj, nil
	}

	if !h.isServerUrlReady() {
		logrus.Info("[scc.registration-controller]: Server URL not set")
		return registrationObj, errors.New("no server url found in the system info")
	}

	// TODO: set a status so we know this is currently processing
	var err error
	// 2. Verify contents of Registration (mode and credentials),
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

	// 4. At the end of either process the current Registration is either:
	// 		a) fulfilled, b) expired or c) failed (retry?)
	// TODO: do we need to do anything here if expired/failed?

	if registrationObj.Status.RegistrationStatus.RequestProcessedTS != "" {
		h.registrations.Enqueue(registrationObj.Name)
	}

	// 4+. If it was a success, then a new Registration is created (and the old one deleted or marked as not current?)
	return registrationObj, nil
}

func (h *handler) setReconcilingCondition(request *v1.Registration, originalErr error) (*v1.Registration, error) {
	logrus.Info("[scc.registration-controller]: set reconciling condition")
	logrus.Error(originalErr)

	// TODO Update status
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
		v1.ResourceConditionProgressing.SetStatusBool(updBackup, false)

		_, err = h.registrations.UpdateStatus(updBackup)
		return err
	})
	if err != nil {
		return request, errors.New(originalErr.Error() + err.Error())
	}

	return request, originalErr
}

func (h *handler) isServerUrlReady() bool {
	return h.systemInfo.ServerUrl() != ""
}
