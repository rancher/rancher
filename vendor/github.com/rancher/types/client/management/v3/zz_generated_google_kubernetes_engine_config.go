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
	ClusterIpv4Cidr          string            `json:"clusterIpv4Cidr,omitempty"`
	Credential               string            `json:"credential,omitempty"`
	Description              string            `json:"description,omitempty"`
	DiskSizeGb               *int64            `json:"diskSizeGb,omitempty"`
	EnableAlphaFeature       *bool             `json:"enableAlphaFeature,omitempty"`
	HTTPLoadBalancing        *bool             `json:"httpLoadBalancing,omitempty"`
	HorizontalPodAutoscaling *bool             `json:"horizontalPodAutoscaling,omitempty"`
	ImageType                string            `json:"imageType,omitempty"`
	KubernetesDashboard      *bool             `json:"kubernetesDashboard,omitempty"`
	Labels                   map[string]string `json:"labels,omitempty"`
	LegacyAbac               *bool             `json:"legacyAbac,omitempty"`
	Locations                []string          `json:"locations,omitempty"`
	MachineType              string            `json:"machineType,omitempty"`
	MasterVersion            string            `json:"masterVersion,omitempty"`
	Network                  string            `json:"network,omitempty"`
	NetworkPolicyConfig      *bool             `json:"networkPolicyConfig,omitempty"`
	NodeCount                *int64            `json:"nodeCount,omitempty"`
	NodeVersion              string            `json:"nodeVersion,omitempty"`
	ProjectID                string            `json:"projectId,omitempty"`
	SubNetwork               string            `json:"subNetwork,omitempty"`
	Zone                     string            `json:"zone,omitempty"`
}
