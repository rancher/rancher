package sccregistration

import (
	"context"
	sccv1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	h := &handler{
		ctx,
	}

	wContext.SCC.RegistrationRequest().OnChange(ctx, "sccRegistrationRequests", h.OnChange)

	return nil
}

type handler struct {
	ctx context.Context
}

func (h *handler) OnChange(key string, registrationRequest *sccv1.RegistrationRequest) (*sccv1.RegistrationRequest, error) {
	return registrationRequest, nil
}
