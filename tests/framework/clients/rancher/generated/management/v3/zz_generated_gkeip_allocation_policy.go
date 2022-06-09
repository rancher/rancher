package client

const (
	GKEIPAllocationPolicyType                            = "gkeipAllocationPolicy"
	GKEIPAllocationPolicyFieldClusterIpv4CidrBlock       = "clusterIpv4CidrBlock"
	GKEIPAllocationPolicyFieldClusterSecondaryRangeName  = "clusterSecondaryRangeName"
	GKEIPAllocationPolicyFieldCreateSubnetwork           = "createSubnetwork"
	GKEIPAllocationPolicyFieldNodeIpv4CidrBlock          = "nodeIpv4CidrBlock"
	GKEIPAllocationPolicyFieldServicesIpv4CidrBlock      = "servicesIpv4CidrBlock"
	GKEIPAllocationPolicyFieldServicesSecondaryRangeName = "servicesSecondaryRangeName"
	GKEIPAllocationPolicyFieldSubnetworkName             = "subnetworkName"
	GKEIPAllocationPolicyFieldUseIPAliases               = "useIpAliases"
)

type GKEIPAllocationPolicy struct {
	ClusterIpv4CidrBlock       string `json:"clusterIpv4CidrBlock,omitempty" yaml:"clusterIpv4CidrBlock,omitempty"`
	ClusterSecondaryRangeName  string `json:"clusterSecondaryRangeName,omitempty" yaml:"clusterSecondaryRangeName,omitempty"`
	CreateSubnetwork           bool   `json:"createSubnetwork,omitempty" yaml:"createSubnetwork,omitempty"`
	NodeIpv4CidrBlock          string `json:"nodeIpv4CidrBlock,omitempty" yaml:"nodeIpv4CidrBlock,omitempty"`
	ServicesIpv4CidrBlock      string `json:"servicesIpv4CidrBlock,omitempty" yaml:"servicesIpv4CidrBlock,omitempty"`
	ServicesSecondaryRangeName string `json:"servicesSecondaryRangeName,omitempty" yaml:"servicesSecondaryRangeName,omitempty"`
	SubnetworkName             string `json:"subnetworkName,omitempty" yaml:"subnetworkName,omitempty"`
	UseIPAliases               bool   `json:"useIpAliases,omitempty" yaml:"useIpAliases,omitempty"`
}
