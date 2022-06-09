package client

const (
	GKEClusterConfigSpecType                                = "gkeClusterConfigSpec"
	GKEClusterConfigSpecFieldClusterAddons                  = "clusterAddons"
	GKEClusterConfigSpecFieldClusterIpv4CidrBlock           = "clusterIpv4Cidr"
	GKEClusterConfigSpecFieldClusterName                    = "clusterName"
	GKEClusterConfigSpecFieldDescription                    = "description"
	GKEClusterConfigSpecFieldEnableKubernetesAlpha          = "enableKubernetesAlpha"
	GKEClusterConfigSpecFieldGoogleCredentialSecret         = "googleCredentialSecret"
	GKEClusterConfigSpecFieldIPAllocationPolicy             = "ipAllocationPolicy"
	GKEClusterConfigSpecFieldImported                       = "imported"
	GKEClusterConfigSpecFieldKubernetesVersion              = "kubernetesVersion"
	GKEClusterConfigSpecFieldLabels                         = "labels"
	GKEClusterConfigSpecFieldLocations                      = "locations"
	GKEClusterConfigSpecFieldLoggingService                 = "loggingService"
	GKEClusterConfigSpecFieldMaintenanceWindow              = "maintenanceWindow"
	GKEClusterConfigSpecFieldMasterAuthorizedNetworksConfig = "masterAuthorizedNetworks"
	GKEClusterConfigSpecFieldMonitoringService              = "monitoringService"
	GKEClusterConfigSpecFieldNetwork                        = "network"
	GKEClusterConfigSpecFieldNetworkPolicyEnabled           = "networkPolicyEnabled"
	GKEClusterConfigSpecFieldNodePools                      = "nodePools"
	GKEClusterConfigSpecFieldPrivateClusterConfig           = "privateClusterConfig"
	GKEClusterConfigSpecFieldProjectID                      = "projectID"
	GKEClusterConfigSpecFieldRegion                         = "region"
	GKEClusterConfigSpecFieldSubnetwork                     = "subnetwork"
	GKEClusterConfigSpecFieldZone                           = "zone"
)

type GKEClusterConfigSpec struct {
	ClusterAddons                  *GKEClusterAddons                  `json:"clusterAddons,omitempty" yaml:"clusterAddons,omitempty"`
	ClusterIpv4CidrBlock           *string                            `json:"clusterIpv4Cidr,omitempty" yaml:"clusterIpv4Cidr,omitempty"`
	ClusterName                    string                             `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	Description                    string                             `json:"description,omitempty" yaml:"description,omitempty"`
	EnableKubernetesAlpha          *bool                              `json:"enableKubernetesAlpha,omitempty" yaml:"enableKubernetesAlpha,omitempty"`
	GoogleCredentialSecret         string                             `json:"googleCredentialSecret,omitempty" yaml:"googleCredentialSecret,omitempty"`
	IPAllocationPolicy             *GKEIPAllocationPolicy             `json:"ipAllocationPolicy,omitempty" yaml:"ipAllocationPolicy,omitempty"`
	Imported                       bool                               `json:"imported,omitempty" yaml:"imported,omitempty"`
	KubernetesVersion              *string                            `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	Labels                         map[string]string                  `json:"labels,omitempty" yaml:"labels,omitempty"`
	Locations                      []string                           `json:"locations,omitempty" yaml:"locations,omitempty"`
	LoggingService                 *string                            `json:"loggingService,omitempty" yaml:"loggingService,omitempty"`
	MaintenanceWindow              *string                            `json:"maintenanceWindow,omitempty" yaml:"maintenanceWindow,omitempty"`
	MasterAuthorizedNetworksConfig *GKEMasterAuthorizedNetworksConfig `json:"masterAuthorizedNetworks,omitempty" yaml:"masterAuthorizedNetworks,omitempty"`
	MonitoringService              *string                            `json:"monitoringService,omitempty" yaml:"monitoringService,omitempty"`
	Network                        *string                            `json:"network,omitempty" yaml:"network,omitempty"`
	NetworkPolicyEnabled           *bool                              `json:"networkPolicyEnabled,omitempty" yaml:"networkPolicyEnabled,omitempty"`
	NodePools                      []GKENodePoolConfig                `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	PrivateClusterConfig           *GKEPrivateClusterConfig           `json:"privateClusterConfig,omitempty" yaml:"privateClusterConfig,omitempty"`
	ProjectID                      string                             `json:"projectID,omitempty" yaml:"projectID,omitempty"`
	Region                         string                             `json:"region,omitempty" yaml:"region,omitempty"`
	Subnetwork                     *string                            `json:"subnetwork,omitempty" yaml:"subnetwork,omitempty"`
	Zone                           string                             `json:"zone,omitempty" yaml:"zone,omitempty"`
}
