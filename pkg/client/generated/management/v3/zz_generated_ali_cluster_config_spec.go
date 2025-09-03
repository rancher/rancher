package client

const (
	AliClusterConfigSpecType                         = "aliClusterConfigSpec"
	AliClusterConfigSpecFieldAddons                  = "addons"
	AliClusterConfigSpecFieldAlibabaCredentialSecret = "alibabaCredentialSecret"
	AliClusterConfigSpecFieldClusterID               = "clusterId"
	AliClusterConfigSpecFieldClusterIsUpgrading      = "clusterIsUpgrading"
	AliClusterConfigSpecFieldClusterName             = "clusterName"
	AliClusterConfigSpecFieldClusterSpec             = "clusterSpec"
	AliClusterConfigSpecFieldClusterType             = "clusterType"
	AliClusterConfigSpecFieldContainerCIDR           = "containerCidr"
	AliClusterConfigSpecFieldEndpointPublicAccess    = "endpointPublicAccess"
	AliClusterConfigSpecFieldImported                = "imported"
	AliClusterConfigSpecFieldKubernetesVersion       = "kubernetesVersion"
	AliClusterConfigSpecFieldNodeCIDRMask            = "nodeCidrMask"
	AliClusterConfigSpecFieldNodePools               = "nodePools"
	AliClusterConfigSpecFieldPauseClusterUpgrade     = "pauseClusterUpgrade"
	AliClusterConfigSpecFieldPodVswitchIDs           = "podVswitchIds"
	AliClusterConfigSpecFieldProxyMode               = "proxyMode"
	AliClusterConfigSpecFieldRegionID                = "regionId"
	AliClusterConfigSpecFieldResourceGroupID         = "resourceGroupId"
	AliClusterConfigSpecFieldSNATEntry               = "snatEntry"
	AliClusterConfigSpecFieldSSHFlags                = "sshFlags"
	AliClusterConfigSpecFieldSecurityGroupID         = "securityGroupId"
	AliClusterConfigSpecFieldServiceCIDR             = "serviceCidr"
	AliClusterConfigSpecFieldTaskID                  = "taskId"
	AliClusterConfigSpecFieldVSwitchIDs              = "vswitchIds"
	AliClusterConfigSpecFieldVpcID                   = "vpcId"
)

type AliClusterConfigSpec struct {
	Addons                  []Addon    `json:"addons,omitempty" yaml:"addons,omitempty"`
	AlibabaCredentialSecret string     `json:"alibabaCredentialSecret,omitempty" yaml:"alibabaCredentialSecret,omitempty"`
	ClusterID               string     `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ClusterIsUpgrading      bool       `json:"clusterIsUpgrading,omitempty" yaml:"clusterIsUpgrading,omitempty"`
	ClusterName             string     `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	ClusterSpec             string     `json:"clusterSpec,omitempty" yaml:"clusterSpec,omitempty"`
	ClusterType             string     `json:"clusterType,omitempty" yaml:"clusterType,omitempty"`
	ContainerCIDR           string     `json:"containerCidr,omitempty" yaml:"containerCidr,omitempty"`
	EndpointPublicAccess    bool       `json:"endpointPublicAccess,omitempty" yaml:"endpointPublicAccess,omitempty"`
	Imported                bool       `json:"imported,omitempty" yaml:"imported,omitempty"`
	KubernetesVersion       string     `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	NodeCIDRMask            int64      `json:"nodeCidrMask,omitempty" yaml:"nodeCidrMask,omitempty"`
	NodePools               []NodePool `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	PauseClusterUpgrade     bool       `json:"pauseClusterUpgrade,omitempty" yaml:"pauseClusterUpgrade,omitempty"`
	PodVswitchIDs           []string   `json:"podVswitchIds,omitempty" yaml:"podVswitchIds,omitempty"`
	ProxyMode               string     `json:"proxyMode,omitempty" yaml:"proxyMode,omitempty"`
	RegionID                string     `json:"regionId,omitempty" yaml:"regionId,omitempty"`
	ResourceGroupID         string     `json:"resourceGroupId,omitempty" yaml:"resourceGroupId,omitempty"`
	SNATEntry               bool       `json:"snatEntry,omitempty" yaml:"snatEntry,omitempty"`
	SSHFlags                bool       `json:"sshFlags,omitempty" yaml:"sshFlags,omitempty"`
	SecurityGroupID         string     `json:"securityGroupId,omitempty" yaml:"securityGroupId,omitempty"`
	ServiceCIDR             string     `json:"serviceCidr,omitempty" yaml:"serviceCidr,omitempty"`
	TaskID                  string     `json:"taskId,omitempty" yaml:"taskId,omitempty"`
	VSwitchIDs              []string   `json:"vswitchIds,omitempty" yaml:"vswitchIds,omitempty"`
	VpcID                   string     `json:"vpcId,omitempty" yaml:"vpcId,omitempty"`
}
