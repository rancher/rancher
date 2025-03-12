package registrationrequest

import (
	"context"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	registrationControllers "github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io/v1"
)

type handler struct {
	ctx                  context.Context
	registrationRequests registrationControllers.RegistrationRequestController
}

func Register(
	ctx context.Context,
	registrationRequests registrationControllers.RegistrationRequestController,
) {
	controller := &handler{
		ctx:                  ctx,
		registrationRequests: registrationRequests,
	}

	registrationRequests.OnChange(ctx, "registrationRequests", controller.OnRegistrationRequestChange)
}

func (h *handler) OnRegistrationRequestChange(_ string, registrationRequest *v1.RegistrationRequest) (*v1.RegistrationRequest, error) {
	return registrationRequest, nil
}
