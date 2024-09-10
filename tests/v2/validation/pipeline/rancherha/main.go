package main

import (
	"strings"

	"github.com/rancher/rancher/tests/v2/actions/pipeline"
	"github.com/rancher/rancher/tests/v2/validation/pipeline/rancherha/corralha"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rkecli"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
)

const rke1 = "rke1"

func main() {
	corralRancherHA := new(corralha.CorralRancherHA)
	config.LoadConfig(corralha.CorralRancherHAConfigConfigurationFileKey, corralRancherHA)

	corralSession := session.NewSession()

	corralConfig := corral.Configurations()
	err := corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	if err != nil {
		logrus.Fatalf("error setting up corral: %v", err)
	}

	configPackage := corral.PackagesConfig()

	environmentFlags := environmentflag.NewEnvironmentFlags()
	environmentflag.LoadEnvironmentFlags(environmentflag.ConfigurationFileKey, environmentFlags)
	installRancher := environmentFlags.GetValue(environmentflag.InstallRancher)

	logrus.Infof("installRancher value is %t", installRancher)

	if installRancher {
		path := configPackage.CorralPackageImages[corralRancherHA.Name]
		corralName := corralRancherHA.Name

		_, err = corral.CreateCorral(corralSession, corralName, path, true, configPackage.HasCleanup)
		if err != nil {
			logrus.Errorf("error creating corral: %v", err)
		}

		bootstrapPassword, err := corral.GetCorralEnvVar(corralName, "bootstrap_password")
		if err != nil {
			logrus.Errorf("error getting the bootstrap password: %v", err)
		}

		rancherConfig := new(rancher.Config)
		config.LoadConfig(rancher.ConfigurationFileKey, rancherConfig)

		token, err := pipeline.CreateAdminToken(bootstrapPassword, rancherConfig)
		if err != nil {
			logrus.Errorf("error creating the admin token: %v", err)
		}

		rancherConfig.AdminToken = token
		config.UpdateConfig(rancher.ConfigurationFileKey, rancherConfig)
		rancherSession := session.NewSession()
		client, err := rancher.NewClient(rancherConfig.AdminToken, rancherSession)
		if err != nil {
			logrus.Errorf("error creating the rancher client: %v", err)
		}

		err = pipeline.PostRancherInstall(client, rancherConfig.AdminPassword)
		if err != nil {
			logrus.Errorf("error during post rancher install: %v", err)
		}

		if strings.Contains(configPackage.CorralPackageImages[corralRancherHA.Name], rke1) {
			sshkey, err := corral.GetCorralEnvVar(corralName, "corral_private_key")
			if err != nil {
				logrus.Errorf("error getting the private SSH key: %v", err)
			}

			rkecliConfig := new(rkecli.Config)
			config.LoadAndUpdateConfig(rkecli.ConfigurationFileKey, rkecliConfig, func() {
				rkecliConfig.SSHKey = sshkey
			})
		}
	} else {
		logrus.Infof("Skipped Rancher Install because installRancher is %t", installRancher)
	}
}
