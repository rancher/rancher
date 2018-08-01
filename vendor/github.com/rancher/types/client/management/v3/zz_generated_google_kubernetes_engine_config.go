package client

const (
	GoogleKubernetesEngineConfigType                                = "googleKubernetesEngineConfig"
	GoogleKubernetesEngineConfigFieldClusterIpv4Cidr                = "clusterIpv4Cidr"
	GoogleKubernetesEngineConfigFieldCredential                     = "credential"
	GoogleKubernetesEngineConfigFieldDescription                    = "description"
	GoogleKubernetesEngineConfigFieldDiskSizeGb                     = "diskSizeGb"
	GoogleKubernetesEngineConfigFieldEnableAlphaFeature             = "enableAlphaFeature"
	GoogleKubernetesEngineConfigFieldEnableHTTPLoadBalancing        = "enableHttpLoadBalancing"
	GoogleKubernetesEngineConfigFieldEnableHorizontalPodAutoscaling = "enableHorizontalPodAutoscaling"
	GoogleKubernetesEngineConfigFieldEnableKubernetesDashboard      = "enableKubernetesDashboard"
	GoogleKubernetesEngineConfigFieldEnableLegacyAbac               = "enableLegacyAbac"
	GoogleKubernetesEngineConfigFieldEnableNetworkPolicyConfig      = "enableNetworkPolicyConfig"
	GoogleKubernetesEngineConfigFieldEnableStackdriverLogging       = "enableStackdriverLogging"
	GoogleKubernetesEngineConfigFieldEnableStackdriverMonitoring    = "enableStackdriverMonitoring"
	GoogleKubernetesEngineConfigFieldImageType                      = "imageType"
	GoogleKubernetesEngineConfigFieldLabels                         = "labels"
	GoogleKubernetesEngineConfigFieldLocations                      = "locations"
	GoogleKubernetesEngineConfigFieldMachineType                    = "machineType"
	GoogleKubernetesEngineConfigFieldMaintenanceWindow              = "maintenanceWindow"
	GoogleKubernetesEngineConfigFieldMasterVersion                  = "masterVersion"
	GoogleKubernetesEngineConfigFieldNetwork                        = "network"
	GoogleKubernetesEngineConfigFieldNodeCount                      = "nodeCount"
	GoogleKubernetesEngineConfigFieldNodeVersion                    = "nodeVersion"
	GoogleKubernetesEngineConfigFieldProjectID                      = "projectId"
	GoogleKubernetesEngineConfigFieldSubNetwork                     = "subNetwork"
	GoogleKubernetesEngineConfigFieldZone                           = "zone"
)

type GoogleKubernetesEngineConfig struct {
	ClusterIpv4Cidr                string            `json:"clusterIpv4Cidr,omitempty" yaml:"clusterIpv4Cidr,omitempty"`
	Credential                     string            `json:"credential,omitempty" yaml:"credential,omitempty"`
	Description                    string            `json:"description,omitempty" yaml:"description,omitempty"`
	DiskSizeGb                     int64             `json:"diskSizeGb,omitempty" yaml:"diskSizeGb,omitempty"`
	EnableAlphaFeature             bool              `json:"enableAlphaFeature,omitempty" yaml:"enableAlphaFeature,omitempty"`
	EnableHTTPLoadBalancing        *bool             `json:"enableHttpLoadBalancing,omitempty" yaml:"enableHttpLoadBalancing,omitempty"`
	EnableHorizontalPodAutoscaling *bool             `json:"enableHorizontalPodAutoscaling,omitempty" yaml:"enableHorizontalPodAutoscaling,omitempty"`
	EnableKubernetesDashboard      bool              `json:"enableKubernetesDashboard,omitempty" yaml:"enableKubernetesDashboard,omitempty"`
	EnableLegacyAbac               bool              `json:"enableLegacyAbac,omitempty" yaml:"enableLegacyAbac,omitempty"`
	EnableNetworkPolicyConfig      *bool             `json:"enableNetworkPolicyConfig,omitempty" yaml:"enableNetworkPolicyConfig,omitempty"`
	EnableStackdriverLogging       *bool             `json:"enableStackdriverLogging,omitempty" yaml:"enableStackdriverLogging,omitempty"`
	EnableStackdriverMonitoring    *bool             `json:"enableStackdriverMonitoring,omitempty" yaml:"enableStackdriverMonitoring,omitempty"`
	ImageType                      string            `json:"imageType,omitempty" yaml:"imageType,omitempty"`
	Labels                         map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Locations                      []string          `json:"locations,omitempty" yaml:"locations,omitempty"`
	MachineType                    string            `json:"machineType,omitempty" yaml:"machineType,omitempty"`
	MaintenanceWindow              string            `json:"maintenanceWindow,omitempty" yaml:"maintenanceWindow,omitempty"`
	MasterVersion                  string            `json:"masterVersion,omitempty" yaml:"masterVersion,omitempty"`
	Network                        string            `json:"network,omitempty" yaml:"network,omitempty"`
	NodeCount                      int64             `json:"nodeCount,omitempty" yaml:"nodeCount,omitempty"`
	NodeVersion                    string            `json:"nodeVersion,omitempty" yaml:"nodeVersion,omitempty"`
	ProjectID                      string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SubNetwork                     string            `json:"subNetwork,omitempty" yaml:"subNetwork,omitempty"`
	Zone                           string            `json:"zone,omitempty" yaml:"zone,omitempty"`
}
