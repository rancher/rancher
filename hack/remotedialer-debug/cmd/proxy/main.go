package main

import (
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"

	"github.com/rancher/remotedialer/proxy"
)

func main() {
	logrus.Info("Starting Remote Dialer Proxy")

	cfg, err := proxy.ConfigFromEnvironment()
	if err != nil {
		logrus.Fatalf("fatal configuration error: %v", err)
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		logrus.Errorf("failed to get in-cluster config: %s", err.Error())
		return
	}

	err = proxy.Start(cfg, restConfig)
	if err != nil {
		logrus.Fatal(err)
	}
}
