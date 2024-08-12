package main

import (
	"fmt"
	"os"

	"github.com/rancher/rancher/tests/v2/actions/pipeline"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/upgradeinput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/rancher/shepherd/pkg/file"
	"github.com/sirupsen/logrus"
)

var (
	configEnvironmentKey = "CATTLE_TEST_CONFIG"
	adminToken           = os.Getenv("HA_TOKEN")
	configPath           = os.Getenv(configEnvironmentKey)
)

const (
	dirName = "cattle-configs"

	localFileName = "local"

	nodeProviderFileName = "node"
	customFileName       = "custom"

	rke1FileName = "rke1"
	rke2FileName = "rke2"
	k3sFileName  = "k3s"

	eksFileName = "eks"
	gkeFileName = "gke"
	aksFileName = "aks"
)

func main() {
	os.Setenv(configEnvironmentKey, configPath)
	defer os.Unsetenv(configEnvironmentKey)

	haConfig := new(pipeline.HAConfig)
	config.LoadConfig(pipeline.HAConfigKey, haConfig)

	rancherConfig := new(rancher.Config)
	config.LoadConfig(rancher.ConfigurationFileKey, rancherConfig)

	if haConfig.Host != "" {
		rancherConfig.Host = haConfig.Host
		rancherConfig.AdminToken = adminToken

		//Rancher cleanup has to be false for the future steps to prevent resource deletion
		rancherCleanup := false
		rancherConfig.Cleanup = &rancherCleanup

		//HA cleanup default to true if not specified
		if haConfig.Cleanup == nil {
			HACleanup := true
			haConfig.Cleanup = &HACleanup
		}

		//Rancher insecure default to true if not specified
		if haConfig.Insecure == nil {
			insecure := true
			rancherConfig.Insecure = &insecure
			haConfig.Insecure = &insecure
		} else {
			rancherConfig.Insecure = haConfig.Insecure
		}

		config.UpdateConfig(rancher.ConfigurationFileKey, rancherConfig)
		config.UpdateConfig(pipeline.HAConfigKey, haConfig)
	}

	testCases := new(pipeline.TestCases)
	config.LoadAndUpdateConfig(pipeline.TestCasesConfigKey, testCases, func() {
		//Upgrade package and tests
		testCases.UpgradeTestPackage = "upgrade"
		testCases.PreUpgradeTestCase = `-run \"TestWorkloadUpgradeTestSuite/TestWorkloadPreUpgrade\"`
		testCases.PostUpgradeTestCase = `-run \"TestWorkloadUpgradeTestSuite/TestWorkloadPostUpgrade\"`
		testCases.UpgradeKubernetesTestCase = `-run \"TestKubernetesUpgradeTestSuite/TestUpgradeKubernetes\"`
	})

	environmentFlags := new(environmentflag.Config)
	config.LoadConfig(environmentflag.ConfigurationFileKey, environmentFlags)

	//Overwrite/update flag to grab cluster names that are provisioned
	environmentFlags.DesiredFlags += "|" + environmentflag.UpdateClusterName.String()

	config.UpdateConfig(environmentflag.ConfigurationFileKey, environmentFlags)

	//make cattle-configs dir
	err := file.NewDir(dirName)
	if err != nil {
		logrus.Fatal("error while creating configs dir", err)
	}

	//copy common configuration for individual configs
	copiedConfig, err := os.ReadFile(configPath)
	if err != nil {
		logrus.Fatal("error while copying upgrade config", err)
	}

	clusters := new(pipeline.Clusters)
	config.LoadConfig(pipeline.ClustersConfigKey, clusters)

	local := clusters.Local
	if local != nil {
		newConfigName := config.NewConfigFileName(dirName, localFileName)
		err := NewRancherLocalClusterConfiguration(*local, newConfigName, copiedConfig)
		if err != nil {
			logrus.Info("error while generating a rancher cluster config", err)
		}
	}

	for i, v := range clusters.RKE1.Custom {
		const isCustom = true
		const isRKE1 = true
		const isRKE2 = false

		for _, cni := range v.CNIs {
			testPackage := "provisioning/rke1"
			runCommand := pipeline.WrapWithAdminRunCommand("TestCustomClusterRKE1ProvisioningTestSuite/TestProvisioningRKE1CustomClusterDynamicInput")
			newConfigName := config.NewConfigFileName(dirName, rke1FileName, customFileName, v.Provider, cni, fmt.Sprint(i))
			err := NewRancherClusterConfiguration(v, newConfigName, isCustom, isRKE1, isRKE2, copiedConfig, cni, testPackage, runCommand, v.Tags, v.RunFlag)
			if err != nil {
				logrus.Info("error while generating a rancher cluster config", err)
				continue
			}
		}
	}

	for i, v := range clusters.RKE1.NodeProvider {
		const isCustom = false
		const isRKE1 = true
		const isRKE2 = false

		for _, cni := range v.CNIs {
			testPackage := "provisioning/rke1"
			runCommand := pipeline.WrapWithAdminRunCommand("TestRKE1ProvisioningTestSuite/TestProvisioningRKE1ClusterDynamicInput")
			newConfigName := config.NewConfigFileName(dirName, rke1FileName, nodeProviderFileName, v.Provider, cni, fmt.Sprint(i))
			err := NewRancherClusterConfiguration(v, newConfigName, isCustom, isRKE1, isRKE2, copiedConfig, cni, testPackage, runCommand, v.Tags, v.RunFlag)
			if err != nil {
				logrus.Info("error while generating a rancher cluster config", err)
				continue
			}
		}
	}

	for i, v := range clusters.RKE2.Custom {
		const isCustom = true
		const isRKE1 = false
		const isRKE2 = true

		for _, cni := range v.CNIs {
			testPackage := "provisioning/rke2"
			runCommand := pipeline.WrapWithAdminRunCommand("TestCustomClusterRKE2ProvisioningTestSuite/TestProvisioningRKE2CustomClusterDynamicInput")
			newConfigName := config.NewConfigFileName(dirName, rke2FileName, customFileName, v.Provider, cni, fmt.Sprint(i))
			err := NewRancherClusterConfiguration(v, newConfigName, isCustom, isRKE1, isRKE2, copiedConfig, cni, testPackage, runCommand, v.Tags, v.RunFlag)
			if err != nil {
				logrus.Info("error while generating a rancher cluster config", err)
				continue
			}
		}
	}

	for i, v := range clusters.RKE2.NodeProvider {
		const isCustom = false
		const isRKE1 = false
		const isRKE2 = true

		for _, cni := range v.CNIs {
			testPackage := "provisioning/rke2"
			runCommand := pipeline.WrapWithAdminRunCommand("TestRKE2ProvisioningTestSuite/TestProvisioningRKE2ClusterDynamicInput")
			newConfigName := config.NewConfigFileName(dirName, rke2FileName, nodeProviderFileName, v.Provider, cni, fmt.Sprint(i))
			err := NewRancherClusterConfiguration(v, newConfigName, isCustom, isRKE1, isRKE2, copiedConfig, cni, testPackage, runCommand, v.Tags, v.RunFlag)
			if err != nil {
				logrus.Info("error while generating a rancher cluster config", err)
				continue
			}
		}
	}

	for i, v := range clusters.K3s.Custom {
		const isCustom = true
		const isRKE1 = false
		const isRKE2 = false

		for _, cni := range v.CNIs {
			testPackage := "provisioning/k3s"
			runCommand := pipeline.WrapWithAdminRunCommand("TestCustomClusterK3SProvisioningTestSuite/TestProvisioningK3SCustomClusterDynamicInput")
			newConfigName := config.NewConfigFileName(dirName, k3sFileName, customFileName, v.Provider, cni, fmt.Sprint(i))
			err := NewRancherClusterConfiguration(v, newConfigName, isCustom, isRKE1, isRKE2, copiedConfig, cni, testPackage, runCommand, v.Tags, v.RunFlag)
			if err != nil {
				logrus.Info("error while generating a rancher cluster config", err)
				continue
			}
		}
	}

	for i, v := range clusters.K3s.NodeProvider {
		const isCustom = false
		const isRKE1 = false
		const isRKE2 = false

		for _, cni := range v.CNIs {
			testPackage := "provisioning/k3s"
			runCommand := pipeline.WrapWithAdminRunCommand("TestK3SProvisioningTestSuite/TestProvisioningK3SClusterDynamicInput")
			newConfigName := config.NewConfigFileName(dirName, k3sFileName, nodeProviderFileName, v.Provider, cni, fmt.Sprint(i))
			err := NewRancherClusterConfiguration(v, newConfigName, isCustom, isRKE1, isRKE2, copiedConfig, cni, testPackage, runCommand, v.Tags, v.RunFlag)
			if err != nil {
				logrus.Info("error while generating a rancher cluster config", err)
				continue
			}
		}
	}

	for i, v := range clusters.Hosted {
		var newConfigName file.Name

		switch v.Provider {
		case provisioninginput.AWSProviderName.String():
			newConfigName = config.NewConfigFileName(dirName, v.Provider, eksFileName, fmt.Sprint(i))
		case provisioninginput.AzureProviderName.String():
			newConfigName = config.NewConfigFileName(dirName, v.Provider, aksFileName, fmt.Sprint(i))
		case provisioninginput.GoogleProviderName.String():
			newConfigName = config.NewConfigFileName(dirName, v.Provider, gkeFileName, fmt.Sprint(i))
		default:
			continue
		}

		newConfigName.NewFile(copiedConfig)

		newConfigName.SetEnvironmentKey(config.ConfigEnvironmentKey)

		pipeline.UpdateHostedKubernetesVField(v.Provider, v.KubernetesVersion)

		hostedTestPackage := "provisioning/hosted/"

		switch v.Provider {
		case provisioninginput.AWSProviderName.String():
			config.LoadAndUpdateConfig(pipeline.TestCasesConfigKey, testCases, func() {
				testCases.ProvisioningTestPackage = hostedTestPackage + "eks"
				runCommand := "TestHostedEKSClusterProvisioningTestSuite/TestProvisioningHostedEKS"
				testCases.ProvisioningTestCase = pipeline.WrapWithAdminRunCommand(runCommand)
			})
		case provisioninginput.AzureProviderName.String():
			config.LoadAndUpdateConfig(pipeline.TestCasesConfigKey, testCases, func() {
				testCases.ProvisioningTestPackage = hostedTestPackage + "aks"
				runCommand := "TestHostedAKSClusterProvisioningTestSuite/TestProvisioningHostedAKS"
				testCases.ProvisioningTestCase = pipeline.WrapWithAdminRunCommand(runCommand)
			})
		case provisioninginput.GoogleProviderName.String():
			config.LoadAndUpdateConfig(pipeline.TestCasesConfigKey, testCases, func() {
				testCases.ProvisioningTestPackage = hostedTestPackage + "gke"
				runCommand := "TestHostedGKEClusterProvisioningTestSuite/TestProvisioningHostedGKE"
				testCases.ProvisioningTestCase = pipeline.WrapWithAdminRunCommand(runCommand)
			})
		default:
			continue
		}

		upgradeConfig := new(upgradeinput.Config)
		config.LoadAndUpdateConfig(upgradeinput.ConfigurationFileKey, upgradeConfig, func() {
			clusters := []upgradeinput.Cluster{
				{
					VersionToUpgrade: v.KubernetesVersionToUpgrade,
					FeaturesToTest:   v.FeaturesToTest,
				},
			}
			upgradeConfig.Clusters = clusters
		})
	}
}

//	NewRancherClusterConfiguration is a function that accepts single cluster from the cluster matrix.
//
// Writes a new configuration file with the original configuration's content and updates the new file's content with the given cluster values.
func NewRancherClusterConfiguration(cluster pipeline.RancherCluster, newConfigName file.Name, isCustom, isRKE1, isRKE2 bool, copiedConfig []byte, cni, provTestPackage, runCommand, tags, runFlag string) (err error) {
	_, err = newConfigName.NewFile(copiedConfig)
	if err != nil {
		logrus.Info("error while writing populated config", err)
		return err
	}

	err = newConfigName.SetEnvironmentKey(config.ConfigEnvironmentKey)
	if err != nil {
		logrus.Info("error while setting new config as env var", err)
		return err
	}

	provisioningConfig := new(provisioninginput.Config)
	config.LoadAndUpdateConfig(provisioninginput.ConfigurationFileKey, provisioningConfig, func() {
		provisioningConfig.CNIs = []string{cni}
		if isRKE1 {
			provisioningConfig.RKE1KubernetesVersions = []string{cluster.KubernetesVersion}
		} else if isRKE2 {
			provisioningConfig.RKE2KubernetesVersions = []string{cluster.KubernetesVersion}
		} else {
			provisioningConfig.K3SKubernetesVersions = []string{cluster.KubernetesVersion}
		}
	})

	testCases := new(pipeline.TestCases)
	config.LoadAndUpdateConfig(pipeline.TestCasesConfigKey, testCases, func() {
		testCases.ProvisioningTestPackage = provTestPackage
		testCases.ProvisioningTestCase = runCommand
		testCases.Tags = tags
	})

	pipeline.UpdateRancherDownstreamClusterFields(&cluster, isCustom, isRKE1)

	upgradeConfig := new(upgradeinput.Config)
	config.LoadAndUpdateConfig(upgradeinput.ConfigurationFileKey, upgradeConfig, func() {
		clusters := []upgradeinput.Cluster{
			{
				VersionToUpgrade: cluster.KubernetesVersionToUpgrade,
				FeaturesToTest:   cluster.FeaturesToTest,
			},
		}
		upgradeConfig.Clusters = clusters
	})

	return
}

//	NewRancherLocalClusterConfiguration is a function that accepts single cluster from the cluster matrix's local field.
//
// Writes a new configuration file with the original configuration's content and updates the new file's content with the given cluster values.
func NewRancherLocalClusterConfiguration(cluster pipeline.RancherCluster, newConfigName file.Name, copiedConfig []byte) (err error) {
	_, err = newConfigName.NewFile(copiedConfig)
	if err != nil {
		logrus.Info("error while writing populated config", err)
		return err
	}

	err = newConfigName.SetEnvironmentKey(config.ConfigEnvironmentKey)
	if err != nil {
		logrus.Info("error while setting new config as env var", err)
		return err
	}

	clusterID := "local"
	upgradeConfig := new(upgradeinput.Config)
	config.LoadAndUpdateConfig(upgradeinput.ConfigurationFileKey, upgradeConfig, func() {
		clusters := []upgradeinput.Cluster{
			{
				Name:             clusterID,
				VersionToUpgrade: cluster.KubernetesVersionToUpgrade,
				FeaturesToTest:   cluster.FeaturesToTest,
			},
		}
		upgradeConfig.Clusters = clusters
	})

	return nil
}
