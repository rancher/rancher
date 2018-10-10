package cluster

import (
	"context"

	clusterController "github.com/rancher/rancher/pkg/controllers/user"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

func RunControllers() error {
	logrus.Info("Starting user controllers")
	c, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	userOnly, err := config.NewUserOnlyContext(*c)
	if err != nil {
		return err
	}

	err = clusterController.RegisterUserOnly(context.Background(), userOnly)
	if err != nil {
		return err
	}

	err = userOnly.Start(context.Background())
	if err != nil {
		return err
	}

	return nil
}
