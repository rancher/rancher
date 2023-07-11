package pipeline

import (
	"github.com/rancher/rancher/tests/framework/clients/ec2"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/aks"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/eks"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/gke"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
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
	case provisioning.AWSProviderName.String():
		provisioningConfig := new(provisioning.Config)
		config.LoadAndUpdateConfig(provisioning.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioning.AWSProviderName.String()}
			if isCustom {
				provisioningConfig.NodeProviders = []string{"ec2"}
			}
		})
	case provisioning.AzureProviderName.String():
		provisioningConfig := new(provisioning.Config)
		config.LoadAndUpdateConfig(provisioning.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioning.AzureProviderName.String()}
		})
	case provisioning.DOProviderName.String():
		provisioningConfig := new(provisioning.Config)
		config.LoadAndUpdateConfig(provisioning.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioning.DOProviderName.String()}
		})
	case provisioning.HarvesterProviderName.String():
		provisioningConfig := new(provisioning.Config)
		config.LoadAndUpdateConfig(provisioning.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioning.HarvesterProviderName.String()}
		})
	case provisioning.LinodeProviderName.String():
		provisioningConfig := new(provisioning.Config)
		config.LoadAndUpdateConfig(provisioning.ConfigurationFileKey, provisioningConfig, func() {
			provisioningConfig.Providers = []string{provisioning.LinodeProviderName.String()}
		})
	default:
		logrus.Error("Couldn't match provider")
	}
}

// UpdateRKE1ImageFields is function that updates the cattle config's node template ssh and image fields
// depending on the provider type.
func UpdateRKE1ImageFields(provider, image, sshUser, volumeType string, isCustom bool) {
	switch provider {
	case provisioning.AWSProviderName.String():
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
	case provisioning.AzureProviderName.String():
		nodeTemplate := new(nodetemplates.AzureNodeTemplateConfig)
		config.LoadAndUpdateConfig(nodetemplates.AzureNodeTemplateConfigurationFileKey, nodeTemplate, func() {
			nodeTemplate.Image = image
			nodeTemplate.SSHUser = sshUser
		})
	case provisioning.HarvesterProviderName.String():
		nodeTemplate := new(nodetemplates.HarvesterNodeTemplateConfig)
		config.LoadAndUpdateConfig(nodetemplates.HarvesterNodeTemplateConfigurationFileKey, nodeTemplate, func() {
			nodeTemplate.ImageName = image
			nodeTemplate.SSHUser = sshUser
		})
	case provisioning.LinodeProviderName.String():
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
	case provisioning.AWSProviderName.String():
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
	case provisioning.AzureProviderName.String():
		machineConfig := new(machinepools.AzureMachineConfig)
		config.LoadAndUpdateConfig(machinepools.AzureMachineConfigConfigurationFileKey, machineConfig, func() {
			machineConfig.Image = image
			machineConfig.SSHUser = sshUser
		})
	case provisioning.DOProviderName.String():
		machineConfig := new(machinepools.DOMachineConfig)
		config.LoadAndUpdateConfig(machinepools.DOMachineConfigConfigurationFileKey, machineConfig, func() {
			machineConfig.Image = image
			machineConfig.SSHUser = sshUser
		})
	case provisioning.HarvesterProviderName.String():
		machineConfig := new(machinepools.HarvesterMachineConfig)
		config.LoadAndUpdateConfig(machinepools.HarvesterMachineConfigConfigurationFileKey, machineConfig, func() {
			machineConfig.ImageName = image
			machineConfig.SSHUser = sshUser
		})
	case provisioning.LinodeProviderName.String():
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
	case provisioning.AWSProviderName.String():
		eksClusterConfig := new(eks.EKSClusterConfig)
		config.LoadAndUpdateConfig(eks.EKSClusterConfigConfigurationFileKey, eksClusterConfig, func() {
			eksClusterConfig.KubernetesVersion = &kubernetesVersion
		})
	case provisioning.AzureProviderName.String():
		aksClusterConfig := new(aks.AKSClusterConfig)
		config.LoadAndUpdateConfig(aks.AKSClusterConfigConfigurationFileKey, aksClusterConfig, func() {
			aksClusterConfig.KubernetesVersion = &kubernetesVersion
		})
	case provisioning.GoogleProviderName.String():
		gkeClusterConfig := new(gke.GKEClusterConfig)
		config.LoadAndUpdateConfig(gke.GKEClusterConfigConfigurationFileKey, gkeClusterConfig, func() {
			gkeClusterConfig.KubernetesVersion = &kubernetesVersion
		})
	}
}
