package main

import (
	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
)

func main() {
	testSession := session.NewSession()

	corralConfig := corral.CorralConfigurations()
	err := corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	if err != nil {
		logrus.Fatalf("error setting up corral: %v", err)
	}
	configPackage := corral.CorralPackagesConfig()

	path := configPackage.CorralPackageImages["aws-rke2-rancher-calico-v1.23.6-rke2r1-2.6.7"]
	_, err = corral.CreateCorral(testSession, "ranchertestcoverage", path, true, configPackage.Cleanup)
	if err != nil {
		logrus.Errorf("error creating corral: %v", err)
	}
}
