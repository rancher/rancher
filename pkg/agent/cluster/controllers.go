// +build !windows

package cluster

import (
	"context"

	"github.com/rancher/rancher/pkg/agent/cluster/controllers"
	clusterController "github.com/rancher/rancher/pkg/controllers/user"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

var running bool

func RunControllers(namespace, token, url string) error {
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

	err = clusterController.RegisterUserOnly(context.Background(), userOnly)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	err = userOnly.Start(ctx)
	if err != nil {
		return err
	}

	if namespace != "" {
		logrus.Infof("Starting agent controllers for namespace [%s], url [%s]", namespace, url)
		if err := controllers.StartControllers(ctx, token, url, namespace); err != nil {
			cancel()
			return err
		}
	}

	running = true
	return nil
}
