// +build !windows

package cluster

import (
	"context"
	"io/ioutil"
	"strings"

	"github.com/rancher/steve/pkg/server"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

const (
	nsFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

func runSteve(ctx context.Context, webhookURL string) error {
	logrus.Info("Starting steve")
	c, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	ns, err := ioutil.ReadFile(nsFile)
	if err != nil {
		return err
	}

	// u, err := url.Parse(webhookURL)
	// if err != nil {
	// 	return err
	// }

	s := server.Server{
		RestConfig: c,
		Namespace:  strings.TrimSpace(string(ns)),
	}

	go func() {
		err := s.ListenAndServe(ctx, 8443, 8080, nil)
		logrus.Fatalf("steve exited: %v", err)
	}()

	return nil
}
