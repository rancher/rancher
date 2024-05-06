package main

import (
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
)

func main() {
	testSession := session.NewSession()

	corralConfig := corral.Configurations()
	err := corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	if err != nil {
		logrus.Fatalf("error setting up corral: %v", err)
	}
	configPackage := corral.PackagesConfig()

	path := configPackage.CorralPackageImages["ranchertestcoverage"]
	_, err = corral.CreateCorral(testSession, "ranchertestcoverage", path, true, configPackage.HasCleanup)
	if err != nil {
		logrus.Errorf("error creating corral: %v", err)
	}
}
