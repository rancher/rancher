package gke

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const (
	// The json/yaml config key for the GKE hosted cluster config
	GKEClusterConfigConfigurationFileKey = "gkeClusterConfig"
)

// ClusterConfig is the configuration needed to create an GKE host cluster
type ClusterConfig struct {
	ClusterAddons                  *ClusterAddons                  `json:"clusterAddons,omitempty" yaml:"clusterAddons,omitempty"`
	ClusterIpv4CidrBlock           *string                         `json:"clusterIpv4Cidr,omitempty" yaml:"clusterIpv4Cidr,omitempty"`
	EnableKubernetesAlpha          *bool                           `json:"enableKubernetesAlpha,omitempty" yaml:"enableKubernetesAlpha,omitempty"`
	IPAllocationPolicy             *IPAllocationPolicy             `json:"ipAllocationPolicy,omitempty" yaml:"ipAllocationPolicy,omitempty"`
	KubernetesVersion              *string                         `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	Labels                         map[string]string               `json:"labels" yaml:"labels"`
	Locations                      []string                        `json:"locations" yaml:"locations"`
	LoggingService                 *string                         `json:"loggingService,omitempty" yaml:"loggingService,omitempty"`
	MaintenanceWindow              *string                         `json:"maintenanceWindow,omitempty" yaml:"maintenanceWindow,omitempty"`
	MasterAuthorizedNetworksConfig *MasterAuthorizedNetworksConfig `json:"masterAuthorizedNetworks,omitempty" yaml:"masterAuthorizedNetworks,omitempty"`
	MonitoringService              *string                         `json:"monitoringService,omitempty" yaml:"monitoringService,omitempty"`
	Network                        *string                         `json:"network,omitempty" yaml:"network,omitempty"`
	NetworkPolicyEnabled           *bool                           `json:"networkPolicyEnabled,omitempty" yaml:"networkPolicyEnabled,omitempty"`
	NodePools                      []NodePool                      `json:"nodePools" yaml:"nodePools"`
	PrivateClusterConfig           *PrivateClusterConfig           `json:"privateClusterConfig,omitempty" yaml:"privateClusterConfig,omitempty"`
	ProjectID                      string                          `json:"projectID" yaml:"projectID"`
	Region                         string                          `json:"region" yaml:"region"`
	Subnetwork                     *string                         `json:"subnetwork,omitempty" yaml:"subnetwork,omitempty"`
	Zone                           string                          `json:"zone" yaml:"zone"`
}

// ClusterAddons is the configuration for the ClusterConfig ClusterAddons
type ClusterAddons struct {
	HTTPLoadBalancing        bool `json:"httpLoadBalancing" yaml:"httpLoadBalancing"`
	HorizontalPodAutoscaling bool `json:"horizontalPodAutoscaling" yaml:"horizontalPodAutoscaling"`
	NetworkPolicyConfig      bool `json:"networkPolicyConfig" yaml:"networkPolicyConfig"`
}

// IPAllocationPolicy is the configuration for the ClusterConfig IPAllocationPolicy
type IPAllocationPolicy struct {
	ClusterIpv4CidrBlock       string `json:"clusterIpv4CidrBlock" yaml:"clusterIpv4CidrBlock"`
	ClusterSecondaryRangeName  string `json:"clusterSecondaryRangeName" yaml:"clusterSecondaryRangeName"`
	CreateSubnetwork           bool   `json:"createSubnetwork" yaml:"createSubnetwork"`
	NodeIpv4CidrBlock          string `json:"nodeIpv4CidrBlock" yaml:"nodeIpv4CidrBlock"`
	ServicesIpv4CidrBlock      string `json:"servicesIpv4CidrBlock" yaml:"servicesIpv4CidrBlock"`
	ServicesSecondaryRangeName string `json:"servicesSecondaryRangeName" yaml:"servicesSecondaryRangeName"`
	SubnetworkName             string `json:"subnetworkName" yaml:"subnetworkName"`
	UseIPAliases               bool   `json:"useIpAliases" yaml:"useIpAliases"`
}

// MasterAuthorizedNetworksConfig is the configuration for the ClusterConfig MasterAuthorizedNetworksConfig
type MasterAuthorizedNetworksConfig struct {
	CidrBlocks []CidrBlock `json:"cidrBlocks" yaml:"cidrBlocks"`
	Enabled    bool        `json:"enabled" yaml:"enabled"`
}

// CidrBlock is the configuration needed for the MasterAuthorizedNetworksConfig CidrBlock
type CidrBlock struct {
	CidrBlock   string `json:"cidrBlock" yaml:"cidrBlock"`
	DisplayName string `json:"displayName" yaml:"displayName"`
}

// NodePool is the configuration needed for the ClusterConfig NodePools
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
	Enabled      bool  `json:"enabled" yaml:"enabled"`
	MaxNodeCount int64 `json:"maxNodeCount" yaml:"maxNodeCount"`
	MinNodeCount int64 `json:"minNodeCount" yaml:"minNodeCount"`
}

// NodeConfig is the configuration needed for the NodePool NodeConfig
type NodeConfig struct {
	DiskSizeGb    int64             `json:"diskSizeGb" yaml:"diskSizeGb"`
	DiskType      string            `json:"diskType" yaml:"diskType"`
	ImageType     string            `json:"imageType" yaml:"imageType"`
	Labels        map[string]string `json:"labels" yaml:"labels"`
	LocalSsdCount int64             `json:"localSsdCount" yaml:"localSsdCount"`
	MachineType   string            `json:"machineType" yaml:"machineType"`
	OauthScopes   []string          `json:"oauthScopes" yaml:"oauthScopes"`
	Preemptible   bool              `json:"preemptible" yaml:"preemptible"`
	Tags          []string          `json:"tags" yaml:"tags"`
	Taints        []NodeTaintConfig `json:"taints" yaml:"taints"`
}

// NodeTaintConfig is the configuration needed for the NodeConfig Taints
type NodeTaintConfig struct {
	Effect string `json:"effect" yaml:"effect"`
	Key    string `json:"key" yaml:"key"`
	Value  string `json:"value" yaml:"value"`
}

// NodePoolManagement is the configuration needed for the NodePool Management
type NodePoolManagement struct {
	AutoRepair  bool `json:"autoRepair" yaml:"autoRepair"`
	AutoUpgrade bool `json:"autoUpgrade" yaml:"autoUpgrade"`
}

// PrivateClusterConfig is the configuration needed for the ClusterConfig PrivateClusterConfig
type PrivateClusterConfig struct {
	EnablePrivateEndpoint bool   `json:"enablePrivateEndpoint" yaml:"enablePrivateEndpoint"`
	EnablePrivateNodes    bool   `json:"enablePrivateNodes" yaml:"enablePrivateNodes"`
	MasterIpv4CidrBlock   string `json:"masterIpv4CidrBlock" yaml:"masterIpv4CidrBlock"`
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
	var gkeClusterConfig ClusterConfig
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
