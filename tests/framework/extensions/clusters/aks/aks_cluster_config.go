package aks

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const (
	AKSClusterConfigConfigurationFileKey = "aksClusterConfig"
)

type AKSClusterConfig struct {
	AuthorizedIPRanges          *[]string          `json:"authorizedIpRanges,omitempty" yaml:"authorizedIpRanges,omitempty"`
	AzureCredentialSecret       string             `json:"azureCredentialSecret,omitempty" yaml:"azureCredentialSecret,omitempty"`
	DNSPrefix                   *string            `json:"dnsPrefix,omitempty" yaml:"dnsPrefix,omitempty"`
	KubernetesVersion           *string            `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	LinuxAdminUsername          *string            `json:"linuxAdminUsername,omitempty" yaml:"linuxAdminUsername,omitempty"`
	LinuxSSHPublicKey           *string            `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	LoadBalancerSKU             *string            `json:"loadBalancerSku,omitempty" yaml:"loadBalancerSku,omitempty"`
	LogAnalyticsWorkspaceGroup  *string            `json:"logAnalyticsWorkspaceGroup,omitempty" yaml:"logAnalyticsWorkspaceGroup,omitempty"`
	LogAnalyticsWorkspaceName   *string            `json:"logAnalyticsWorkspaceName,omitempty" yaml:"logAnalyticsWorkspaceName,omitempty"`
	Monitoring                  *bool              `json:"monitoring,omitempty" yaml:"monitoring,omitempty"`
	NetworkDNSServiceIP         *string            `json:"dnsServiceIp,omitempty" yaml:"dnsServiceIp,omitempty"`
	NetworkDockerBridgeCIDR     *string            `json:"dockerBridgeCidr,omitempty" yaml:"dockerBridgeCidr,omitempty"`
	NetworkPlugin               *string            `json:"networkPlugin,omitempty" yaml:"networkPlugin,omitempty"`
	NetworkPodCIDR              *string            `json:"podCidr,omitempty" yaml:"podCidr,omitempty"`
	NetworkPolicy               *string            `json:"networkPolicy,omitempty" yaml:"networkPolicy,omitempty"`
	NetworkServiceCIDR          *string            `json:"serviceCidr,omitempty" yaml:"serviceCidr,omitempty"`
	NodePools                   *[]AKSNodePool     `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	PrivateCluster              *bool              `json:"privateCluster,omitempty" yaml:"privateCluster,omitempty"`
	ResourceGroup               string             `json:"resourceGroup,omitempty" yaml:"resourceGroup,omitempty"`
	ResourceLocation            string             `json:"resourceLocation,omitempty" yaml:"resourceLocation,omitempty"`
	Subnet                      *string            `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	Tags                        *map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	VirtualNetwork              *string            `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
	VirtualNetworkResourceGroup *string            `json:"virtualNetworkResourceGroup,omitempty" yaml:"virtualNetworkResourceGroup,omitempty"`
}

type AKSNodePool struct {
	AvailabilityZones   *[]string `json:"availabilityZones,omitempty" yaml:"availabilityZones,omitempty"`
	NodeCount           *int64    `json:"nodeCount,omitempty" yaml:"nodeCount,omitempty"`
	EnableAutoScaling   *bool     `json:"enableAutoScaling,omitempty" yaml:"enableAutoScaling,omitempty"`
	MaxPods             *int64    `json:"maxPods,omitempty" yaml:"maxPods,omitempty"`
	Mode                string    `json:"mode,omitempty" yaml:"mode,omitempty"`
	Name                *string   `json:"name,omitempty" yaml:"name,omitempty"`
	OrchestratorVersion *string   `json:"orchestratorVersion,omitempty" yaml:"orchestratorVersion,omitempty"`
	OsDiskSizeGB        *int64    `json:"osDiskSizeGB,omitempty" yaml:"osDiskSizeGB,omitempty"`
	OsDiskType          string    `json:"osDiskType,omitempty" yaml:"osDiskType,omitempty"`
	OsType              string    `json:"osType,omitempty" yaml:"osType,omitempty"`
	VMSize              string    `json:"vmSize,omitempty" yaml:"vmSize,omitempty"`
}

func aksNodePoolConstructor(aksNodePoolConfigs *[]AKSNodePool, kubernetesVersion string) *[]management.AKSNodePool {
	var aksNodePools []management.AKSNodePool
	for _, aksNodePoolConfig := range *aksNodePoolConfigs {
		aksNodePool := management.AKSNodePool{
			AvailabilityZones:   aksNodePoolConfig.AvailabilityZones,
			Count:               aksNodePoolConfig.NodeCount,
			EnableAutoScaling:   aksNodePoolConfig.EnableAutoScaling,
			MaxPods:             aksNodePoolConfig.MaxPods,
			Mode:                aksNodePoolConfig.Mode,
			Name:                aksNodePoolConfig.Name,
			OrchestratorVersion: &kubernetesVersion,
			OsDiskSizeGB:        aksNodePoolConfig.OsDiskSizeGB,
			OsDiskType:          aksNodePoolConfig.OsDiskType,
			OsType:              aksNodePoolConfig.OsType,
			VMSize:              aksNodePoolConfig.VMSize,
		}
		aksNodePools = append(aksNodePools, aksNodePool)
	}
	return &aksNodePools
}

func aksHostClusterConfig(displayName, cloudCredentialID string) *management.AKSClusterConfigSpec {
	var aksClusterConfig AKSClusterConfig
	config.LoadConfig(AKSClusterConfigConfigurationFileKey, &aksClusterConfig)

	return &management.AKSClusterConfigSpec{
		AzureCredentialSecret: cloudCredentialID,
		ClusterName:           displayName,
		DNSPrefix:             aksClusterConfig.DNSPrefix,
		Imported:              false,
		NodePools:             aksNodePoolConstructor(aksClusterConfig.NodePools, *aksClusterConfig.KubernetesVersion),
		KubernetesVersion:     aksClusterConfig.KubernetesVersion,
		LinuxAdminUsername:    aksClusterConfig.LinuxAdminUsername,
		LoadBalancerSKU:       aksClusterConfig.LoadBalancerSKU,
		NetworkPlugin:         aksClusterConfig.NetworkPlugin,
		PrivateCluster:        aksClusterConfig.PrivateCluster,
		ResourceGroup:         aksClusterConfig.ResourceGroup,
		ResourceLocation:      aksClusterConfig.ResourceLocation,
		Tags:                  aksClusterConfig.Tags,
	}
}
