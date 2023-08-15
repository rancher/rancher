package pipeline

import (
	"github.com/rancher/rancher/tests/framework/clients/ec2"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/aks"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/eks"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/gke"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/sirupsen/logrus"
)

// UpdateRancherDownstreamClusterFields is function that updates the cattle config's providers, node providers,
// ssh user and image fields depending on the provider and cluster types.
func UpdateRancherDownstreamClusterFields(cluster *RancherCluster, isCustom, isRKE1 bool) {
	UpdateProviderField(cluster.Provider, isCustom)

	if isRKE1 {
		UpdateRKE1ImageFields(cluster.Provider, cluster.Image, cluster.SSHUser, cluster.VolumeType, isCustom)
	} else {
		UpdateRKE2ImageFields(cluster.Provider, cluster.Image, cluster.SSHUser, cluster.VolumeType, isCustom)
	}
}

// UpdateProviderField is function that updates the cattle config's provisioning input providers field
// and if custom, additionally updates nodeProviders field.
func UpdateProviderField(provider string, isCustom bool) {
	switch provider {
	case provisioninginput.AWSProviderName.String():
		provisioningConfig := new(provisioninginput.Config)
		config.LoadAndUpdateConfig(provisioninginput.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioninginput.AWSProviderName.String()}
			if isCustom {
				provisioningConfig.NodeProviders = []string{"ec2"}
			}
		})
	case provisioninginput.AzureProviderName.String():
		provisioningConfig := new(provisioninginput.Config)
		config.LoadAndUpdateConfig(provisioninginput.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioninginput.AzureProviderName.String()}
		})
	case provisioninginput.DOProviderName.String():
		provisioningConfig := new(provisioninginput.Config)
		config.LoadAndUpdateConfig(provisioninginput.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioninginput.DOProviderName.String()}
		})
	case provisioninginput.HarvesterProviderName.String():
		provisioningConfig := new(provisioninginput.Config)
		config.LoadAndUpdateConfig(provisioninginput.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioninginput.HarvesterProviderName.String()}
		})
	case provisioninginput.LinodeProviderName.String():
		provisioningConfig := new(provisioninginput.Config)
		config.LoadAndUpdateConfig(provisioninginput.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioninginput.LinodeProviderName.String()}
		})
	default:
		logrus.Error("Couldn't match provider")
	}
}

// UpdateRKE1ImageFields is function that updates the cattle config's node template ssh and image fields
// depending on the provider type.
func UpdateRKE1ImageFields(provider, image, sshUser, volumeType string, isCustom bool) {
	switch provider {
	case provisioninginput.AWSProviderName.String():
		if !isCustom {
			nodeTemplate := new(nodetemplates.AmazonEC2NodeTemplateConfig)
			config.LoadAndUpdateConfig(nodetemplates.AmazonEC2NodeTemplateConfigurationFileKey, nodeTemplate, func() {
				nodeTemplate.AMI = image
				nodeTemplate.SSHUser = sshUser
				nodeTemplate.VolumeType = volumeType
			})
		} else {
			ec2Configs := new(ec2.AWSEC2Configs)
			config.LoadAndUpdateConfig(ec2.ConfigurationFileKey, ec2Configs, func() {
				for i := range ec2Configs.AWSEC2Config {
					ec2Configs.AWSEC2Config[i].AWSAMI = image
					ec2Configs.AWSEC2Config[i].AWSUser = sshUser
				}
			})
		}
	case provisioninginput.AzureProviderName.String():
		nodeTemplate := new(nodetemplates.AzureNodeTemplateConfig)
		config.LoadAndUpdateConfig(nodetemplates.AzureNodeTemplateConfigurationFileKey, nodeTemplate, func() {
			nodeTemplate.Image = image
			nodeTemplate.SSHUser = sshUser
		})
	case provisioninginput.HarvesterProviderName.String():
		nodeTemplate := new(nodetemplates.HarvesterNodeTemplateConfig)
		config.LoadAndUpdateConfig(nodetemplates.HarvesterNodeTemplateConfigurationFileKey, nodeTemplate, func() {
			nodeTemplate.ImageName = image
			nodeTemplate.SSHUser = sshUser
		})
	case provisioninginput.LinodeProviderName.String():
		nodeTemplate := new(nodetemplates.LinodeNodeTemplateConfig)
		config.LoadAndUpdateConfig(nodetemplates.LinodeNodeTemplateConfigurationFileKey, nodeTemplate, func() {
			nodeTemplate.Image = image
			nodeTemplate.SSHUser = sshUser
		})
	default:
		logrus.Error("Couldn't match RKE1 image fields")
	}
}

// UpdateRKE2ImageFields is function that updates the cattle config's node template ssh and image fields
// depending on the provider type.
func UpdateRKE2ImageFields(provider, image, sshUser, volumeType string, isCustom bool) {
	switch provider {
	case provisioninginput.AWSProviderName.String():
		if !isCustom {
			machineConfig := new(machinepools.AWSMachineConfig)
			config.LoadAndUpdateConfig(machinepools.AWSMachineConfigConfigurationFileKey, machineConfig, func() {
				machineConfig.AMI = image
				machineConfig.SSHUser = sshUser
				machineConfig.VolumeType = volumeType
			})
		} else {
			ec2Configs := new(ec2.AWSEC2Configs)
			config.LoadAndUpdateConfig(ec2.ConfigurationFileKey, ec2Configs, func() {
				for i := range ec2Configs.AWSEC2Config {
					ec2Configs.AWSEC2Config[i].AWSAMI = image
					ec2Configs.AWSEC2Config[i].AWSUser = sshUser
				}
			})
		}
	case provisioninginput.AzureProviderName.String():
		machineConfig := new(machinepools.AzureMachineConfig)
		config.LoadAndUpdateConfig(machinepools.AzureMachineConfigConfigurationFileKey, machineConfig, func() {
			machineConfig.Image = image
			machineConfig.SSHUser = sshUser
		})
	case provisioninginput.DOProviderName.String():
		machineConfig := new(machinepools.DOMachineConfig)
		config.LoadAndUpdateConfig(machinepools.DOMachineConfigConfigurationFileKey, machineConfig, func() {
			machineConfig.Image = image
			machineConfig.SSHUser = sshUser
		})
	case provisioninginput.HarvesterProviderName.String():
		machineConfig := new(machinepools.HarvesterMachineConfig)
		config.LoadAndUpdateConfig(machinepools.HarvesterMachineConfigConfigurationFileKey, machineConfig, func() {
			machineConfig.ImageName = image
			machineConfig.SSHUser = sshUser
		})
	case provisioninginput.LinodeProviderName.String():
		machineConfig := new(machinepools.LinodeMachineConfig)
		config.LoadAndUpdateConfig(machinepools.LinodeMachineConfigConfigurationFileKey, machineConfig, func() {
			machineConfig.Image = image
			machineConfig.SSHUser = sshUser
		})
	default:
		logrus.Error("Couldn't match RKE2 image fields")
	}
}

// UpdateHostedKubernetesVField is function that updates the cattle config's hosted cluster kubernetes version field
// depending on the provider type.
func UpdateHostedKubernetesVField(provider, kubernetesVersion string) {
	switch provider {
	case provisioninginput.AWSProviderName.String():
		eksClusterConfig := new(eks.EKSClusterConfig)
		config.LoadAndUpdateConfig(eks.EKSClusterConfigConfigurationFileKey, eksClusterConfig, func() {
			eksClusterConfig.KubernetesVersion = &kubernetesVersion
		})
	case provisioninginput.AzureProviderName.String():
		aksClusterConfig := new(aks.AKSClusterConfig)
		config.LoadAndUpdateConfig(aks.AKSClusterConfigConfigurationFileKey, aksClusterConfig, func() {
			aksClusterConfig.KubernetesVersion = &kubernetesVersion
		})
	case provisioninginput.GoogleProviderName.String():
		gkeClusterConfig := new(gke.GKEClusterConfig)
		config.LoadAndUpdateConfig(gke.GKEClusterConfigConfigurationFileKey, gkeClusterConfig, func() {
			gkeClusterConfig.KubernetesVersion = &kubernetesVersion
		})
	}
}
