// +build !windows

package cluster

import (
	"context"

	"github.com/rancher/steve/pkg/auth"
	"github.com/rancher/steve/pkg/server"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

func runSteve(ctx context.Context, webhookURL string) error {
	logrus.Info("Starting steve")
	c, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	// u, err := url.Parse(webhookURL)
	// if err != nil {
	// 	return err
	// }

	s := server.Server{
		RestConfig:     c,
		AuthMiddleware: auth.ToMiddleware(auth.AuthenticatorFunc(auth.Impersonation)),
	}

	go func() {
		err := s.ListenAndServe(ctx, 8443, 8080, nil)
		logrus.Fatalf("steve exited: %v", err)
	}()

	return nil
}
