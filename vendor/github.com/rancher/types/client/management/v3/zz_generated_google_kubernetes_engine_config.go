package client

const (
	GoogleKubernetesEngineConfigType                                 = "googleKubernetesEngineConfig"
	GoogleKubernetesEngineConfigFieldClusterIpv4Cidr                 = "clusterIpv4Cidr"
	GoogleKubernetesEngineConfigFieldCredential                      = "credential"
	GoogleKubernetesEngineConfigFieldDescription                     = "description"
	GoogleKubernetesEngineConfigFieldDisableHTTPLoadBalancing        = "disableHttpLoadBalancing"
	GoogleKubernetesEngineConfigFieldDisableHorizontalPodAutoscaling = "disableHorizontalPodAutoscaling"
	GoogleKubernetesEngineConfigFieldDisableNetworkPolicyConfig      = "disableNetworkPolicyConfig"
	GoogleKubernetesEngineConfigFieldDiskSizeGb                      = "diskSizeGb"
	GoogleKubernetesEngineConfigFieldEnableAlphaFeature              = "enableAlphaFeature"
	GoogleKubernetesEngineConfigFieldEnableKubernetesDashboard       = "enableKubernetesDashboard"
	GoogleKubernetesEngineConfigFieldEnableLegacyAbac                = "enableLegacyAbac"
	GoogleKubernetesEngineConfigFieldImageType                       = "imageType"
	GoogleKubernetesEngineConfigFieldLabels                          = "labels"
	GoogleKubernetesEngineConfigFieldLocations                       = "locations"
	GoogleKubernetesEngineConfigFieldMachineType                     = "machineType"
	GoogleKubernetesEngineConfigFieldMasterVersion                   = "masterVersion"
	GoogleKubernetesEngineConfigFieldNetwork                         = "network"
	GoogleKubernetesEngineConfigFieldNodeCount                       = "nodeCount"
	GoogleKubernetesEngineConfigFieldNodeVersion                     = "nodeVersion"
	GoogleKubernetesEngineConfigFieldProjectID                       = "projectId"
	GoogleKubernetesEngineConfigFieldSubNetwork                      = "subNetwork"
	GoogleKubernetesEngineConfigFieldZone                            = "zone"
)

type GoogleKubernetesEngineConfig struct {
	ClusterIpv4Cidr                 string            `json:"clusterIpv4Cidr,omitempty" yaml:"clusterIpv4Cidr,omitempty"`
	Credential                      string            `json:"credential,omitempty" yaml:"credential,omitempty"`
	Description                     string            `json:"description,omitempty" yaml:"description,omitempty"`
	DisableHTTPLoadBalancing        bool              `json:"disableHttpLoadBalancing,omitempty" yaml:"disableHttpLoadBalancing,omitempty"`
	DisableHorizontalPodAutoscaling bool              `json:"disableHorizontalPodAutoscaling,omitempty" yaml:"disableHorizontalPodAutoscaling,omitempty"`
	DisableNetworkPolicyConfig      bool              `json:"disableNetworkPolicyConfig,omitempty" yaml:"disableNetworkPolicyConfig,omitempty"`
	DiskSizeGb                      int64             `json:"diskSizeGb,omitempty" yaml:"diskSizeGb,omitempty"`
	EnableAlphaFeature              bool              `json:"enableAlphaFeature,omitempty" yaml:"enableAlphaFeature,omitempty"`
	EnableKubernetesDashboard       bool              `json:"enableKubernetesDashboard,omitempty" yaml:"enableKubernetesDashboard,omitempty"`
	EnableLegacyAbac                bool              `json:"enableLegacyAbac,omitempty" yaml:"enableLegacyAbac,omitempty"`
	ImageType                       string            `json:"imageType,omitempty" yaml:"imageType,omitempty"`
	Labels                          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Locations                       []string          `json:"locations,omitempty" yaml:"locations,omitempty"`
	MachineType                     string            `json:"machineType,omitempty" yaml:"machineType,omitempty"`
	MasterVersion                   string            `json:"masterVersion,omitempty" yaml:"masterVersion,omitempty"`
	Network                         string            `json:"network,omitempty" yaml:"network,omitempty"`
	NodeCount                       int64             `json:"nodeCount,omitempty" yaml:"nodeCount,omitempty"`
	NodeVersion                     string            `json:"nodeVersion,omitempty" yaml:"nodeVersion,omitempty"`
	ProjectID                       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SubNetwork                      string            `json:"subNetwork,omitempty" yaml:"subNetwork,omitempty"`
	Zone                            string            `json:"zone,omitempty" yaml:"zone,omitempty"`
}
