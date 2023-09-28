package aks

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const (
	// The json/yaml config key for the AKS hosted cluster config
	AKSClusterConfigConfigurationFileKey = "aksClusterConfig"
)

// ClusterConfig is the configuration needed to create an AKS host cluster
type ClusterConfig struct {
	AuthorizedIPRanges          *[]string         `json:"authorizedIpRanges,omitempty" yaml:"authorizedIpRanges,omitempty"`
	AzureCredentialSecret       string            `json:"azureCredentialSecret" yaml:"azureCredentialSecret"`
	DNSPrefix                   *string           `json:"dnsPrefix,omitempty" yaml:"dnsPrefix,omitempty"`
	KubernetesVersion           *string           `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	LinuxAdminUsername          *string           `json:"linuxAdminUsername,omitempty" yaml:"linuxAdminUsername,omitempty"`
	LinuxSSHPublicKey           *string           `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	LoadBalancerSKU             *string           `json:"loadBalancerSku,omitempty" yaml:"loadBalancerSku,omitempty"`
	LogAnalyticsWorkspaceGroup  *string           `json:"logAnalyticsWorkspaceGroup,omitempty" yaml:"logAnalyticsWorkspaceGroup,omitempty"`
	LogAnalyticsWorkspaceName   *string           `json:"logAnalyticsWorkspaceName,omitempty" yaml:"logAnalyticsWorkspaceName,omitempty"`
	Monitoring                  *bool             `json:"monitoring,omitempty" yaml:"monitoring,omitempty"`
	NetworkDNSServiceIP         *string           `json:"dnsServiceIp,omitempty" yaml:"dnsServiceIp,omitempty"`
	NetworkDockerBridgeCIDR     *string           `json:"dockerBridgeCidr,omitempty" yaml:"dockerBridgeCidr,omitempty"`
	NetworkPlugin               *string           `json:"networkPlugin,omitempty" yaml:"networkPlugin,omitempty"`
	NetworkPodCIDR              *string           `json:"podCidr,omitempty" yaml:"podCidr,omitempty"`
	NetworkPolicy               *string           `json:"networkPolicy,omitempty" yaml:"networkPolicy,omitempty"`
	NetworkServiceCIDR          *string           `json:"serviceCidr,omitempty" yaml:"serviceCidr,omitempty"`
	NodePools                   *[]NodePool       `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	PrivateCluster              *bool             `json:"privateCluster,omitempty" yaml:"privateCluster,omitempty"`
	ResourceGroup               string            `json:"resourceGroup" yaml:"resourceGroup"`
	ResourceLocation            string            `json:"resourceLocation" yaml:"resourceLocation"`
	Subnet                      *string           `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	Tags                        map[string]string `json:"tags" yaml:"tags"`
	VirtualNetwork              *string           `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
	VirtualNetworkResourceGroup *string           `json:"virtualNetworkResourceGroup,omitempty" yaml:"virtualNetworkResourceGroup,omitempty"`
}

// NodePool is the configuration needed to an AKS node pool
type NodePool struct {
	AvailabilityZones   *[]string `json:"availabilityZones,omitempty" yaml:"availabilityZones,omitempty"`
	EnableAutoScaling   *bool     `json:"enableAutoScaling,omitempty" yaml:"enableAutoScaling,omitempty"`
	MaxPods             *int64    `json:"maxPods,omitempty" yaml:"maxPods,omitempty"`
	MaxCount            *int64    `json:"maxCount,omitempty" yaml:"maxCount,omitempty"`
	MinCount            *int64    `json:"minCount,omitempty" yaml:"minCount,omitempty"`
	Mode                string    `json:"mode" yaml:"mode"`
	Name                *string   `json:"name,omitempty" yaml:"name,omitempty"`
	NodeCount           *int64    `json:"nodeCount,omitempty" yaml:"nodeCount,omitempty"`
	OrchestratorVersion *string   `json:"orchestratorVersion,omitempty" yaml:"orchestratorVersion,omitempty"`
	OsDiskSizeGB        *int64    `json:"osDiskSizeGB,omitempty" yaml:"osDiskSizeGB,omitempty"`
	OsDiskType          string    `json:"osDiskType" yaml:"osDiskType"`
	OsType              string    `json:"osType" yaml:"osType"`
	VMSize              string    `json:"vmSize" yaml:"vmSize"`
}

func aksNodePoolConstructor(aksNodePoolConfigs *[]NodePool, kubernetesVersion string) []management.AKSNodePool {
	var aksNodePools []management.AKSNodePool
	for _, aksNodePoolConfig := range *aksNodePoolConfigs {
		aksNodePool := management.AKSNodePool{
			AvailabilityZones:   aksNodePoolConfig.AvailabilityZones,
			Count:               aksNodePoolConfig.NodeCount,
			EnableAutoScaling:   aksNodePoolConfig.EnableAutoScaling,
			MaxPods:             aksNodePoolConfig.MaxPods,
			MaxCount:            aksNodePoolConfig.MaxCount,
			MinCount:            aksNodePoolConfig.MinCount,
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
	return aksNodePools
}

func HostClusterConfig(displayName, cloudCredentialID string) *management.AKSClusterConfigSpec {
	var aksClusterConfig ClusterConfig
	config.LoadConfig(AKSClusterConfigConfigurationFileKey, &aksClusterConfig)

	return &management.AKSClusterConfigSpec{
		AzureCredentialSecret: cloudCredentialID,
		ClusterName:           displayName,
		DNSPrefix:             aksClusterConfig.DNSPrefix,
		Imported:              false,
		KubernetesVersion:     aksClusterConfig.KubernetesVersion,
		LinuxAdminUsername:    aksClusterConfig.LinuxAdminUsername,
		LoadBalancerSKU:       aksClusterConfig.LoadBalancerSKU,
		NetworkPlugin:         aksClusterConfig.NetworkPlugin,
		NodePools:             aksNodePoolConstructor(aksClusterConfig.NodePools, *aksClusterConfig.KubernetesVersion),
		PrivateCluster:        aksClusterConfig.PrivateCluster,
		ResourceGroup:         aksClusterConfig.ResourceGroup,
		ResourceLocation:      aksClusterConfig.ResourceLocation,
		Tags:                  aksClusterConfig.Tags,
	}
}
