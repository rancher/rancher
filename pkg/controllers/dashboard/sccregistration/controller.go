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
	logrus.Info("Enable SCC registration related stuffs..")
	return nil
}
