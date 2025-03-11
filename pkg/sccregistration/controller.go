package sccregistration

import (
	"context"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

func Register(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	h := &handler{
		ctx,
	}
	wContext.Catalog

	return nil
}

type handler struct {
	ctx context.Context
}
