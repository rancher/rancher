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
	// TODO register controllers here
	logrus.Info("Register controllers here")

	return nil
}
