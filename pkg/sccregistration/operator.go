package sccregistration

import (
	"context"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	h := &handler{
		ctx,
	}

	wContext.Mgmt

	return nil
}

type handler struct {
	ctx context.Context
}
