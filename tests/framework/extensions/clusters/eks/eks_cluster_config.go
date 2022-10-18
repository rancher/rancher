package eks

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const (
	// The json/yaml config key for the EKS hosted cluster config
	EKSClusterConfigConfigurationFileKey = "eksClusterConfig"
)

// EKSClusterConfig is the configuration needed to create an EKS host cluster
type EKSClusterConfig struct {
	KmsKey              *string            `json:"kmsKey,omitempty" yaml:"kmsKey,omitempty"`
	KubernetesVersion   *string            `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	LoggingTypes        []string           `json:"loggingTypes,omitempty" yaml:"loggingTypes,omitempty"`
	NodeGroupsConfig    *[]NodeGroupConfig `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	PrivateAccess       *bool              `json:"privateAccess,omitempty" yaml:"privateAccess,omitempty"`
	PublicAccess        *bool              `json:"publicAccess,omitempty" yaml:"publicAccess,omitempty"`
	PublicAccessSources []string           `json:"publicAccessSources,omitempty" yaml:"publicAccessSources,omitempty"`
	Region              string             `json:"region,omitempty" yaml:"region,omitempty"`
	SecretsEncryption   *bool              `json:"secretsEncryption,omitempty" yaml:"secretsEncryption,omitempty"`
	SecurityGroups      []string           `json:"securityGroups,omitempty" yaml:"securityGroups,omitempty"`
	ServiceRole         *string            `json:"serviceRole,omitempty" yaml:"serviceRole,omitempty"`
	Subnets             []string           `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	Tags                map[string]string  `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// NodeGroupConfig is the configuration need to create an EKS node group
type NodeGroupConfig struct {
	DesiredSize          *int64                `json:"desiredSize,omitempty" yaml:"desiredSize,omitempty"`
	DiskSize             *int64                `json:"diskSize,omitempty" yaml:"diskSize,omitempty"`
	Ec2SshKey            *string               `json:"ec2SshKey,omitempty" yaml:"ec2SshKey,omitempty"`
	Gpu                  *bool                 `json:"gpu,omitempty" yaml:"gpu,omitempty"`
	ImageID              *string               `json:"imageId,omitempty" yaml:"imageId,omitempty"`
	InstanceType         *string               `json:"instanceType,omitempty" yaml:"instanceType,omitempty"`
	Labels               map[string]string     `json:"labels,omitempty" yaml:"labels,omitempty"`
	LaunchTemplateConfig *LaunchTemplateConfig `json:"launchTemplate,omitempty" yaml:"launchTemplate,omitempty"`
	MaxSize              *int64                `json:"maxSize,omitempty" yaml:"maxSize,omitempty"`
	MinSize              *int64                `json:"minSize,omitempty" yaml:"minSize,omitempty"`
	NodegroupName        *string               `json:"nodegroupName,omitempty" yaml:"nodegroupName,omitempty"`
	RequestSpotInstances *bool                 `json:"requestSpotInstances,omitempty" yaml:"requestSpotInstances,omitempty"`
	ResourceTags         map[string]string     `json:"resourceTags,omitempty" yaml:"resourceTags,omitempty"`
	SpotInstanceTypes    []string              `json:"spotInstanceTypes,omitempty" yaml:"spotInstanceTypes,omitempty"`
	Subnets              []string              `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	Tags                 map[string]string     `json:"tags,omitempty" yaml:"tags,omitempty"`
	UserData             *string               `json:"userData,omitempty" yaml:"userData,omitempty"`
}

// LaunchTemplateConfig is the configuration need for a node group launch template
type LaunchTemplateConfig struct {
	Name    *string `json:"name,omitempty" yaml:"name,omitempty"`
	Version *int64  `json:"version,omitempty" yaml:"version,omitempty"`
}

func nodeGroupsConstructor(nodeGroupsConfig *[]NodeGroupConfig, kubernetesVersion string) []management.NodeGroup {
	var nodeGroups []management.NodeGroup
	for _, nodeGroupConfig := range *nodeGroupsConfig {
		var launchTemplate *management.LaunchTemplate
		if nodeGroupConfig.LaunchTemplateConfig != nil {
			launchTemplate = &management.LaunchTemplate{
				Name:    nodeGroupConfig.LaunchTemplateConfig.Name,
				Version: nodeGroupConfig.LaunchTemplateConfig.Version,
			}
		}
		nodeGroup := management.NodeGroup{
			DesiredSize:          nodeGroupConfig.DesiredSize,
			DiskSize:             nodeGroupConfig.DiskSize,
			Ec2SshKey:            nodeGroupConfig.Ec2SshKey,
			Gpu:                  nodeGroupConfig.Gpu,
			ImageID:              nodeGroupConfig.ImageID,
			InstanceType:         nodeGroupConfig.InstanceType,
			Labels:               nodeGroupConfig.Labels,
			LaunchTemplate:       launchTemplate,
			MaxSize:              nodeGroupConfig.MaxSize,
			MinSize:              nodeGroupConfig.MinSize,
			NodegroupName:        nodeGroupConfig.NodegroupName,
			RequestSpotInstances: nodeGroupConfig.RequestSpotInstances,
			ResourceTags:         nodeGroupConfig.ResourceTags,
			SpotInstanceTypes:    nodeGroupConfig.SpotInstanceTypes,
			Subnets:              nodeGroupConfig.Subnets,
			Tags:                 nodeGroupConfig.Tags,
			UserData:             nodeGroupConfig.UserData,
			Version:              &kubernetesVersion,
		}
		nodeGroups = append(nodeGroups, nodeGroup)
	}
	return nodeGroups
}

func eksHostClusterConfig(displayName, cloudCredentialID string) *management.EKSClusterConfigSpec {
	var eksClusterConfig EKSClusterConfig
	config.LoadConfig(EKSClusterConfigConfigurationFileKey, &eksClusterConfig)

	return &management.EKSClusterConfigSpec{
		AmazonCredentialSecret: cloudCredentialID,
		DisplayName:            displayName,
		Imported:               false,
		KmsKey:                 eksClusterConfig.KmsKey,
		KubernetesVersion:      eksClusterConfig.KubernetesVersion,
		LoggingTypes:           eksClusterConfig.LoggingTypes,
		NodeGroups:             nodeGroupsConstructor(eksClusterConfig.NodeGroupsConfig, *eksClusterConfig.KubernetesVersion),
		PrivateAccess:          eksClusterConfig.PrivateAccess,
		PublicAccess:           eksClusterConfig.PublicAccess,
		PublicAccessSources:    eksClusterConfig.PublicAccessSources,
		Region:                 eksClusterConfig.Region,
		SecretsEncryption:      eksClusterConfig.SecretsEncryption,
		SecurityGroups:         eksClusterConfig.SecurityGroups,
		ServiceRole:            eksClusterConfig.ServiceRole,
		Subnets:                eksClusterConfig.Subnets,
		Tags:                   eksClusterConfig.Tags,
	}
}
