package cluster

import (
	"context"

	"github.com/rancher/rancher/pkg/agent/steve"
	"github.com/rancher/rancher/pkg/controllers/managementagent"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

var running bool

func RunControllers(ctx context.Context) error {
	if running {
		return nil
	}

	logrus.Info("Starting user controllers")
	c, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	userOnly, err := config.NewUserOnlyContext(*c)
	if err != nil {
		return err
	}

	if err := managementagent.Register(ctx, userOnly); err != nil {
		return err
	}

	if err := userOnly.Start(ctx); err != nil {
		return err
	}

	if err := steve.Run(ctx); err != nil {
		return err
	}

	running = true
	return nil
}
