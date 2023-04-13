package gke

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const (
	// The json/yaml config key for the GKE hosted cluster config
	GKEClusterConfigConfigurationFileKey = "gkeClusterConfig"
)

// GKEClusterConfig is the configuration needed to create an GKE host cluster
type GKEClusterConfig struct {
	ClusterAddons                  *ClusterAddons                  `json:"clusterAddons,omitempty" yaml:"clusterAddons,omitempty"`
	ClusterIpv4CidrBlock           *string                         `json:"clusterIpv4Cidr,omitempty" yaml:"clusterIpv4Cidr,omitempty"`
	EnableKubernetesAlpha          *bool                           `json:"enableKubernetesAlpha,omitempty" yaml:"enableKubernetesAlpha,omitempty"`
	IPAllocationPolicy             *IPAllocationPolicy             `json:"ipAllocationPolicy,omitempty" yaml:"ipAllocationPolicy,omitempty"`
	KubernetesVersion              *string                         `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	Labels                         map[string]string               `json:"labels,omitempty" yaml:"labels,omitempty"`
	Locations                      []string                        `json:"locations,omitempty" yaml:"locations,omitempty"`
	LoggingService                 *string                         `json:"loggingService,omitempty" yaml:"loggingService,omitempty"`
	MaintenanceWindow              *string                         `json:"maintenanceWindow,omitempty" yaml:"maintenanceWindow,omitempty"`
	MasterAuthorizedNetworksConfig *MasterAuthorizedNetworksConfig `json:"masterAuthorizedNetworks,omitempty" yaml:"masterAuthorizedNetworks,omitempty"`
	MonitoringService              *string                         `json:"monitoringService,omitempty" yaml:"monitoringService,omitempty"`
	Network                        *string                         `json:"network,omitempty" yaml:"network,omitempty"`
	NetworkPolicyEnabled           *bool                           `json:"networkPolicyEnabled,omitempty" yaml:"networkPolicyEnabled,omitempty"`
	NodePools                      []NodePool                      `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	PrivateClusterConfig           *PrivateClusterConfig           `json:"privateClusterConfig,omitempty" yaml:"privateClusterConfig,omitempty"`
	ProjectID                      string                          `json:"projectID,omitempty" yaml:"projectID,omitempty"`
	Region                         string                          `json:"region,omitempty" yaml:"region,omitempty"`
	Subnetwork                     *string                         `json:"subnetwork,omitempty" yaml:"subnetwork,omitempty"`
	Zone                           string                          `json:"zone,omitempty" yaml:"zone,omitempty"`
}

// ClusterAddons is the configuration for the GKEClusterConfig ClusterAddons
type ClusterAddons struct {
	HTTPLoadBalancing        bool `json:"httpLoadBalancing,omitempty" yaml:"httpLoadBalancing,omitempty"`
	HorizontalPodAutoscaling bool `json:"horizontalPodAutoscaling,omitempty" yaml:"horizontalPodAutoscaling,omitempty"`
	NetworkPolicyConfig      bool `json:"networkPolicyConfig,omitempty" yaml:"networkPolicyConfig,omitempty"`
}

// IPAllocationPolicy is the configuration for the GKEClusterConfig IPAllocationPolicy
type IPAllocationPolicy struct {
	ClusterIpv4CidrBlock       string `json:"clusterIpv4CidrBlock,omitempty" yaml:"clusterIpv4CidrBlock,omitempty"`
	ClusterSecondaryRangeName  string `json:"clusterSecondaryRangeName,omitempty" yaml:"clusterSecondaryRangeName,omitempty"`
	CreateSubnetwork           bool   `json:"createSubnetwork,omitempty" yaml:"createSubnetwork,omitempty"`
	NodeIpv4CidrBlock          string `json:"nodeIpv4CidrBlock,omitempty" yaml:"nodeIpv4CidrBlock,omitempty"`
	ServicesIpv4CidrBlock      string `json:"servicesIpv4CidrBlock,omitempty" yaml:"servicesIpv4CidrBlock,omitempty"`
	ServicesSecondaryRangeName string `json:"servicesSecondaryRangeName,omitempty" yaml:"servicesSecondaryRangeName,omitempty"`
	SubnetworkName             string `json:"subnetworkName,omitempty" yaml:"subnetworkName,omitempty"`
	UseIPAliases               bool   `json:"useIpAliases,omitempty" yaml:"useIpAliases,omitempty"`
}

// MasterAuthorizedNetworksConfig is the configuration for the GKEClusterConfig MasterAuthorizedNetworksConfig
type MasterAuthorizedNetworksConfig struct {
	CidrBlocks []CidrBlock `json:"cidrBlocks,omitempty" yaml:"cidrBlocks,omitempty"`
	Enabled    bool        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// CidrBlock is the configuration needed for the MasterAuthorizedNetworksConfig CidrBlock
type CidrBlock struct {
	CidrBlock   string `json:"cidrBlock,omitempty" yaml:"cidrBlock,omitempty"`
	DisplayName string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
}

// NodePool is the configuration needed for the GKEClusterConfig NodePools
type NodePool struct {
	Autoscaling       *Autoscaling        `json:"autoscaling,omitempty" yaml:"autoscaling,omitempty"`
	Config            *NodeConfig         `json:"config,omitempty" yaml:"config,omitempty"`
	InitialNodeCount  *int64              `json:"initialNodeCount,omitempty" yaml:"initialNodeCount,omitempty"`
	Management        *NodePoolManagement `json:"management,omitempty" yaml:"management,omitempty"`
	MaxPodsConstraint *int64              `json:"maxPodsConstraint,omitempty" yaml:"maxPodsConstraint,omitempty"`
	Name              *string             `json:"name,omitempty" yaml:"name,omitempty"`
	Version           *string             `json:"version,omitempty" yaml:"version,omitempty"`
}

// Autoscaling is the configuration needed for the NodePool Autoscaling
type Autoscaling struct {
	Enabled      bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	MaxNodeCount int64 `json:"maxNodeCount,omitempty" yaml:"maxNodeCount,omitempty"`
	MinNodeCount int64 `json:"minNodeCount,omitempty" yaml:"minNodeCount,omitempty"`
}

// NodeConfig is the configuration needed for the NodePool NodeConfig
type NodeConfig struct {
	DiskSizeGb    int64             `json:"diskSizeGb,omitempty" yaml:"diskSizeGb,omitempty"`
	DiskType      string            `json:"diskType,omitempty" yaml:"diskType,omitempty"`
	ImageType     string            `json:"imageType,omitempty" yaml:"imageType,omitempty"`
	Labels        map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LocalSsdCount int64             `json:"localSsdCount,omitempty" yaml:"localSsdCount,omitempty"`
	MachineType   string            `json:"machineType,omitempty" yaml:"machineType,omitempty"`
	OauthScopes   []string          `json:"oauthScopes,omitempty" yaml:"oauthScopes,omitempty"`
	Preemptible   bool              `json:"preemptible,omitempty" yaml:"preemptible,omitempty"`
	Tags          []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Taints        []NodeTaintConfig `json:"taints,omitempty" yaml:"taints,omitempty"`
}

// NodeTaintConfig is the configuration needed for the NodeConfig Taints
type NodeTaintConfig struct {
	Effect string `json:"effect,omitempty" yaml:"effect,omitempty"`
	Key    string `json:"key,omitempty" yaml:"key,omitempty"`
	Value  string `json:"value,omitempty" yaml:"value,omitempty"`
}

// NodePoolManagement is the configuration needed for the NodePool Management
type NodePoolManagement struct {
	AutoRepair  bool `json:"autoRepair,omitempty" yaml:"autoRepair,omitempty"`
	AutoUpgrade bool `json:"autoUpgrade,omitempty" yaml:"autoUpgrade,omitempty"`
}

// PrivateClusterConfig is the configuration needed for the GKEClusterConfig PrivateClusterConfig
type PrivateClusterConfig struct {
	EnablePrivateEndpoint bool   `json:"enablePrivateEndpoint,omitempty" yaml:"enablePrivateEndpoint,omitempty"`
	EnablePrivateNodes    bool   `json:"enablePrivateNodes,omitempty" yaml:"enablePrivateNodes,omitempty"`
	MasterIpv4CidrBlock   string `json:"masterIpv4CidrBlock,omitempty" yaml:"masterIpv4CidrBlock,omitempty"`
}

func clusterAddonsBuilder(clusterAddons *ClusterAddons) *management.GKEClusterAddons {
	return &management.GKEClusterAddons{
		HTTPLoadBalancing:        clusterAddons.HTTPLoadBalancing,
		HorizontalPodAutoscaling: clusterAddons.HorizontalPodAutoscaling,
		NetworkPolicyConfig:      clusterAddons.NetworkPolicyConfig,
	}
}

func ipAllocationPolicyBuilder(ipAllocationPolicy *IPAllocationPolicy) *management.GKEIPAllocationPolicy {
	return &management.GKEIPAllocationPolicy{
		ClusterIpv4CidrBlock:       ipAllocationPolicy.ClusterIpv4CidrBlock,
		ClusterSecondaryRangeName:  ipAllocationPolicy.ClusterSecondaryRangeName,
		CreateSubnetwork:           ipAllocationPolicy.CreateSubnetwork,
		NodeIpv4CidrBlock:          ipAllocationPolicy.NodeIpv4CidrBlock,
		ServicesIpv4CidrBlock:      ipAllocationPolicy.NodeIpv4CidrBlock,
		ServicesSecondaryRangeName: ipAllocationPolicy.ServicesSecondaryRangeName,
		SubnetworkName:             ipAllocationPolicy.SubnetworkName,
		UseIPAliases:               ipAllocationPolicy.UseIPAliases,
	}
}

func masterAuthorizedNetworksConfigBuilder(masterAuthorizedNetworksConfig MasterAuthorizedNetworksConfig) *management.GKEMasterAuthorizedNetworksConfig {
	return &management.GKEMasterAuthorizedNetworksConfig{
		Enabled:    masterAuthorizedNetworksConfig.Enabled,
		CidrBlocks: cidrBlocksBuilder(masterAuthorizedNetworksConfig.CidrBlocks),
	}
}

func cidrBlocksBuilder(cidrBlocks []CidrBlock) []management.GKECidrBlock {
	var newCidrBlocks []management.GKECidrBlock
	for _, circidrBlock := range cidrBlocks {
		gkeCircidrBlock := management.GKECidrBlock{
			CidrBlock:   circidrBlock.CidrBlock,
			DisplayName: circidrBlock.DisplayName,
		}
		newCidrBlocks = append(newCidrBlocks, gkeCircidrBlock)

	}
	return newCidrBlocks
}

func nodePoolsBuilder(nodePools []NodePool, kubernetesVersion *string) []management.GKENodePoolConfig {
	var gkeNodePoolConfigs []management.GKENodePoolConfig
	for _, nodePool := range nodePools {
		gkeNodePoolConfig := management.GKENodePoolConfig{
			Autoscaling:       autoScallingBuilder(nodePool.Autoscaling),
			Config:            nodeConfigBuilder(nodePool.Config),
			InitialNodeCount:  nodePool.InitialNodeCount,
			Management:        nodePoolManagementBuilder(nodePool.Management),
			MaxPodsConstraint: nodePool.MaxPodsConstraint,
			Name:              nodePool.Name,
			Version:           kubernetesVersion,
		}

		gkeNodePoolConfigs = append(gkeNodePoolConfigs, gkeNodePoolConfig)
	}
	return gkeNodePoolConfigs
}

func nodePoolManagementBuilder(nodePoolManagement *NodePoolManagement) *management.GKENodePoolManagement {
	return &management.GKENodePoolManagement{
		AutoRepair:  nodePoolManagement.AutoRepair,
		AutoUpgrade: nodePoolManagement.AutoUpgrade,
	}
}

func nodeConfigBuilder(nodeConfig *NodeConfig) *management.GKENodeConfig {
	return &management.GKENodeConfig{
		DiskSizeGb:    nodeConfig.DiskSizeGb,
		DiskType:      nodeConfig.DiskType,
		ImageType:     nodeConfig.ImageType,
		Labels:        nodeConfig.Labels,
		LocalSsdCount: nodeConfig.LocalSsdCount,
		MachineType:   nodeConfig.MachineType,
		OauthScopes:   nodeConfig.OauthScopes,
		Preemptible:   nodeConfig.Preemptible,
		Tags:          nodeConfig.Tags,
		Taints:        taintsBuilder(nodeConfig.Taints),
	}
}

func autoScallingBuilder(autoScalling *Autoscaling) *management.GKENodePoolAutoscaling {
	return &management.GKENodePoolAutoscaling{
		Enabled:      autoScalling.Enabled,
		MaxNodeCount: autoScalling.MaxNodeCount,
		MinNodeCount: autoScalling.MinNodeCount,
	}
}

func taintsBuilder(taints []NodeTaintConfig) []management.GKENodeTaintConfig {
	var gkeNodeTaintConfigs []management.GKENodeTaintConfig
	for _, taint := range taints {
		gkeNodeTaintConfig := management.GKENodeTaintConfig{
			Effect: taint.Effect,
			Key:    taint.Key,
			Value:  taint.Value,
		}

		gkeNodeTaintConfigs = append(gkeNodeTaintConfigs, gkeNodeTaintConfig)
	}

	return gkeNodeTaintConfigs
}

func privateClusterConfigBuilder(privateClusterConfig *PrivateClusterConfig) *management.GKEPrivateClusterConfig {
	return &management.GKEPrivateClusterConfig{
		EnablePrivateEndpoint: privateClusterConfig.EnablePrivateEndpoint,
		EnablePrivateNodes:    privateClusterConfig.EnablePrivateNodes,
		MasterIpv4CidrBlock:   privateClusterConfig.MasterIpv4CidrBlock,
	}
}

func gkeHostClusterConfig(clusterName, cloudCredentialID string) *management.GKEClusterConfigSpec {
	var gkeClusterConfig GKEClusterConfig
	config.LoadConfig(GKEClusterConfigConfigurationFileKey, &gkeClusterConfig)

	return &management.GKEClusterConfigSpec{
		ClusterAddons:                  clusterAddonsBuilder(gkeClusterConfig.ClusterAddons),
		ClusterIpv4CidrBlock:           gkeClusterConfig.ClusterIpv4CidrBlock,
		ClusterName:                    clusterName,
		EnableKubernetesAlpha:          gkeClusterConfig.EnableKubernetesAlpha,
		GoogleCredentialSecret:         cloudCredentialID,
		Imported:                       false,
		IPAllocationPolicy:             ipAllocationPolicyBuilder(gkeClusterConfig.IPAllocationPolicy),
		KubernetesVersion:              gkeClusterConfig.KubernetesVersion,
		Labels:                         &gkeClusterConfig.Labels,
		Locations:                      &gkeClusterConfig.Locations,
		LoggingService:                 gkeClusterConfig.LoggingService,
		MaintenanceWindow:              gkeClusterConfig.MaintenanceWindow,
		MasterAuthorizedNetworksConfig: masterAuthorizedNetworksConfigBuilder(*gkeClusterConfig.MasterAuthorizedNetworksConfig),
		MonitoringService:              gkeClusterConfig.MonitoringService,
		Network:                        gkeClusterConfig.Network,
		NetworkPolicyEnabled:           gkeClusterConfig.NetworkPolicyEnabled,
		NodePools:                      nodePoolsBuilder(gkeClusterConfig.NodePools, gkeClusterConfig.KubernetesVersion),
		PrivateClusterConfig:           privateClusterConfigBuilder(gkeClusterConfig.PrivateClusterConfig),
		ProjectID:                      gkeClusterConfig.ProjectID,
		Region:                         gkeClusterConfig.Region,
		Subnetwork:                     gkeClusterConfig.Subnetwork,
		Zone:                           gkeClusterConfig.Zone,
	}
}
