package main

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
)

const corralPackageName = "registryrancher"

func main() {
	testSession := session.NewSession()

	corralConfig := corral.CorralConfigurations()
	err := corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	if err != nil {
		logrus.Fatalf("error setting up corral: %v", err)
	}

	configPackage := corral.CorralPackagesConfig()

	path := configPackage.CorralPackageImages[corralPackageName]

	fmt.Println("PATH", path)
	_, err = corral.CreateCorral(testSession, "rancherglobalregistry", path, true, configPackage.Cleanup)
	if err != nil {
		logrus.Errorf("error creating corral: %v", err)
	}
}
