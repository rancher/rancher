package client

const (
	GoogleKubernetesEngineConfigType                          = "googleKubernetesEngineConfig"
	GoogleKubernetesEngineConfigFieldClusterIpv4Cidr          = "clusterIpv4Cidr"
	GoogleKubernetesEngineConfigFieldCredential               = "credential"
	GoogleKubernetesEngineConfigFieldDescription              = "description"
	GoogleKubernetesEngineConfigFieldDiskSizeGb               = "diskSizeGb"
	GoogleKubernetesEngineConfigFieldEnableAlphaFeature       = "enableAlphaFeature"
	GoogleKubernetesEngineConfigFieldHTTPLoadBalancing        = "httpLoadBalancing"
	GoogleKubernetesEngineConfigFieldHorizontalPodAutoscaling = "horizontalPodAutoscaling"
	GoogleKubernetesEngineConfigFieldImageType                = "imageType"
	GoogleKubernetesEngineConfigFieldKubernetesDashboard      = "kubernetesDashboard"
	GoogleKubernetesEngineConfigFieldLabels                   = "labels"
	GoogleKubernetesEngineConfigFieldLegacyAbac               = "legacyAbac"
	GoogleKubernetesEngineConfigFieldLocations                = "locations"
	GoogleKubernetesEngineConfigFieldMachineType              = "machineType"
	GoogleKubernetesEngineConfigFieldMasterVersion            = "masterVersion"
	GoogleKubernetesEngineConfigFieldNetwork                  = "network"
	GoogleKubernetesEngineConfigFieldNetworkPolicyConfig      = "networkPolicyConfig"
	GoogleKubernetesEngineConfigFieldNodeCount                = "nodeCount"
	GoogleKubernetesEngineConfigFieldNodeVersion              = "nodeVersion"
	GoogleKubernetesEngineConfigFieldProjectID                = "projectId"
	GoogleKubernetesEngineConfigFieldSubNetwork               = "subNetwork"
	GoogleKubernetesEngineConfigFieldZone                     = "zone"
)

type GoogleKubernetesEngineConfig struct {
	ClusterIpv4Cidr          string            `json:"clusterIpv4Cidr,omitempty" yaml:"clusterIpv4Cidr,omitempty"`
	Credential               string            `json:"credential,omitempty" yaml:"credential,omitempty"`
	Description              string            `json:"description,omitempty" yaml:"description,omitempty"`
	DiskSizeGb               *int64            `json:"diskSizeGb,omitempty" yaml:"diskSizeGb,omitempty"`
	EnableAlphaFeature       bool              `json:"enableAlphaFeature,omitempty" yaml:"enableAlphaFeature,omitempty"`
	HTTPLoadBalancing        bool              `json:"httpLoadBalancing,omitempty" yaml:"httpLoadBalancing,omitempty"`
	HorizontalPodAutoscaling bool              `json:"horizontalPodAutoscaling,omitempty" yaml:"horizontalPodAutoscaling,omitempty"`
	ImageType                string            `json:"imageType,omitempty" yaml:"imageType,omitempty"`
	KubernetesDashboard      bool              `json:"kubernetesDashboard,omitempty" yaml:"kubernetesDashboard,omitempty"`
	Labels                   map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LegacyAbac               bool              `json:"legacyAbac,omitempty" yaml:"legacyAbac,omitempty"`
	Locations                []string          `json:"locations,omitempty" yaml:"locations,omitempty"`
	MachineType              string            `json:"machineType,omitempty" yaml:"machineType,omitempty"`
	MasterVersion            string            `json:"masterVersion,omitempty" yaml:"masterVersion,omitempty"`
	Network                  string            `json:"network,omitempty" yaml:"network,omitempty"`
	NetworkPolicyConfig      bool              `json:"networkPolicyConfig,omitempty" yaml:"networkPolicyConfig,omitempty"`
	NodeCount                *int64            `json:"nodeCount,omitempty" yaml:"nodeCount,omitempty"`
	NodeVersion              string            `json:"nodeVersion,omitempty" yaml:"nodeVersion,omitempty"`
	ProjectID                string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SubNetwork               string            `json:"subNetwork,omitempty" yaml:"subNetwork,omitempty"`
	Zone                     string            `json:"zone,omitempty" yaml:"zone,omitempty"`
}
