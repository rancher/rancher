package client

const (
	AliClusterConfigSpecType                           = "aliClusterConfigSpec"
	AliClusterConfigSpecFieldAddons                    = "addons"
	AliClusterConfigSpecFieldAlibabaCredentialSecret   = "alibabaCredentialSecret"
	AliClusterConfigSpecFieldClusterID                 = "clusterId"
	AliClusterConfigSpecFieldClusterName               = "clusterName"
	AliClusterConfigSpecFieldClusterSpec               = "clusterSpec"
	AliClusterConfigSpecFieldClusterType               = "clusterType"
	AliClusterConfigSpecFieldContainerCIDR             = "containerCidr"
	AliClusterConfigSpecFieldEndpointPublicAccess      = "endpointPublicAccess"
	AliClusterConfigSpecFieldImported                  = "imported"
	AliClusterConfigSpecFieldIsEnterpriseSecurityGroup = "isEnterpriseSecurityGroup"
	AliClusterConfigSpecFieldKubernetesVersion         = "kubernetesVersion"
	AliClusterConfigSpecFieldNodeCIDRMask              = "nodeCidrMask"
	AliClusterConfigSpecFieldNodePools                 = "nodePools"
	AliClusterConfigSpecFieldPodVswitchIDs             = "podVswitchIds"
	AliClusterConfigSpecFieldProxyMode                 = "proxyMode"
	AliClusterConfigSpecFieldRegionID                  = "regionId"
	AliClusterConfigSpecFieldResourceGroupID           = "resourceGroupId"
	AliClusterConfigSpecFieldSNATEntry                 = "snatEntry"
	AliClusterConfigSpecFieldSecurityGroupID           = "securityGroupId"
	AliClusterConfigSpecFieldServiceCIDR               = "serviceCidr"
	AliClusterConfigSpecFieldVSwitchIDs                = "vswitchIds"
	AliClusterConfigSpecFieldVpcID                     = "vpcId"
	AliClusterConfigSpecFieldZoneIDs                   = "zoneIds"
)

type AliClusterConfigSpec struct {
	Addons                    []AliAddon    `json:"addons,omitempty" yaml:"addons,omitempty"`
	AlibabaCredentialSecret   string        `json:"alibabaCredentialSecret,omitempty" yaml:"alibabaCredentialSecret,omitempty"`
	ClusterID                 string        `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ClusterName               string        `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	ClusterSpec               string        `json:"clusterSpec,omitempty" yaml:"clusterSpec,omitempty"`
	ClusterType               string        `json:"clusterType,omitempty" yaml:"clusterType,omitempty"`
	ContainerCIDR             string        `json:"containerCidr,omitempty" yaml:"containerCidr,omitempty"`
	EndpointPublicAccess      bool          `json:"endpointPublicAccess,omitempty" yaml:"endpointPublicAccess,omitempty"`
	Imported                  bool          `json:"imported,omitempty" yaml:"imported,omitempty"`
	IsEnterpriseSecurityGroup *bool         `json:"isEnterpriseSecurityGroup,omitempty" yaml:"isEnterpriseSecurityGroup,omitempty"`
	KubernetesVersion         string        `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	NodeCIDRMask              int64         `json:"nodeCidrMask,omitempty" yaml:"nodeCidrMask,omitempty"`
	NodePools                 []AliNodePool `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	PodVswitchIDs             []string      `json:"podVswitchIds,omitempty" yaml:"podVswitchIds,omitempty"`
	ProxyMode                 string        `json:"proxyMode,omitempty" yaml:"proxyMode,omitempty"`
	RegionID                  string        `json:"regionId,omitempty" yaml:"regionId,omitempty"`
	ResourceGroupID           string        `json:"resourceGroupId,omitempty" yaml:"resourceGroupId,omitempty"`
	SNATEntry                 bool          `json:"snatEntry,omitempty" yaml:"snatEntry,omitempty"`
	SecurityGroupID           string        `json:"securityGroupId,omitempty" yaml:"securityGroupId,omitempty"`
	ServiceCIDR               string        `json:"serviceCidr,omitempty" yaml:"serviceCidr,omitempty"`
	VSwitchIDs                []string      `json:"vswitchIds,omitempty" yaml:"vswitchIds,omitempty"`
	VpcID                     string        `json:"vpcId,omitempty" yaml:"vpcId,omitempty"`
	ZoneIDs                   []string      `json:"zoneIds,omitempty" yaml:"zoneIds,omitempty"`
}
