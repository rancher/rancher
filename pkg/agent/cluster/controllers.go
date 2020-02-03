// +build !windows

package cluster

import (
	"context"

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

	// ctx, _ := context.WithCancel(context.Background())
	err = userOnly.Start(context.Background())
	if err != nil {
		return err
	}

	// namespace will be if steve is enabled
	if namespace != "" {
		if err := runSteve(context.Background(), url); err != nil {
			return err
		}

		// logrus.Infof("Starting agent controllers for namespace [%s], url [%s]", namespace, url)
		// if err := controllers.StartControllers(ctx, token, url, namespace); err != nil {
		// 	cancel()
		// 	return err
		// }
	}

	running = true
	return nil
}
