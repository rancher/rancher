package main

import (
	"os"

	"github.com/creasty/defaults"
	rancherClient "github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/token"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// setup for integration testing
func main() {
	rancherConfig := new(rancherClient.Config)
	file := "config.yaml"

	user := &management.User{
		Username: "admin",
		Password: "admin",
	}

	hostURL := "localhost:8443"
	token, err := token.GenerateUserToken(user, hostURL)
	if err != nil {
		logrus.Fatalf("error with generating admin token: %v", err)
	}

	cleanup := true
	rancherConfig.AdminToken = token.Token
	rancherConfig.Host = hostURL
	rancherConfig.Cleanup = &cleanup
	rancherConfig.ClusterName = "local"

	if err := defaults.Set(rancherConfig); err != nil {
		logrus.Fatalf("error with setting up config file: %v", err)
	}

	all := map[string]*rancherClient.Config{}
	all[rancherClient.ConfigurationFileKey] = rancherConfig
	yamlConfig, err := yaml.Marshal(all)
	if err != nil {
		logrus.Fatalf("error with marshaling config file: %v", err)
	}

	err = os.WriteFile(file, yamlConfig, 0644)
	if err != nil {
		logrus.Fatalf("error with writing config file: %v", err)
	}
}
