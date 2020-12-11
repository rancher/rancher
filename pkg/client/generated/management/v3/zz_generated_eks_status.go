package client

const (
	EKSStatusType                       = "eksStatus"
	EKSStatusFieldPrivateRequiresTunnel = "privateRequiresTunnel"
	EKSStatusFieldSecurityGroups        = "securityGroups"
	EKSStatusFieldSubnets               = "subnets"
	EKSStatusFieldUpstreamSpec          = "upstreamSpec"
	EKSStatusFieldVirtualNetwork        = "virtualNetwork"
)

type EKSStatus struct {
	PrivateRequiresTunnel *bool                 `json:"privateRequiresTunnel,omitempty" yaml:"privateRequiresTunnel,omitempty"`
	SecurityGroups        []string              `json:"securityGroups,omitempty" yaml:"securityGroups,omitempty"`
	Subnets               []string              `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	UpstreamSpec          *EKSClusterConfigSpec `json:"upstreamSpec,omitempty" yaml:"upstreamSpec,omitempty"`
	VirtualNetwork        string                `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
}
