package registration

import (
	"context"
	"github.com/sirupsen/logrus"

	"github.com/rancher/rancher/pkg/sccregistration/suseconnect"

	registrationClient "github.com/SUSE/connect-ng/pkg/registration"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
)

type handler struct {
	ctx           context.Context
	registrations registrationControllers.RegistrationController
}

func Register(
	ctx context.Context,
	registrations registrationControllers.RegistrationController,
) {
	controller := &handler{
		ctx:           ctx,
		registrations: registrations,
	}

	registrations.OnChange(ctx, "registrations", controller.OnRegistrationChange)
}

func (h *handler) OnRegistrationChange(_ string, registration *v1.Registration) (*v1.Registration, error) {

	sccConnection := suseconnect.DefaultRancherConnection()
	activate, p, err := registrationClient.Activate(
		sccConnection,
		"some-identifier",
		"version",
		"arch",
		"regcode",
	)
	if err != nil {
		return nil, err
	}

	logrus.Info(activate)
	logrus.Info(p)

	return registration, nil
}
