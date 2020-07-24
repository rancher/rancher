package client

const (
	EKSStatusType                = "eksStatus"
	EKSStatusFieldSecurityGroups = "securityGroups"
	EKSStatusFieldSubnets        = "subnets"
	EKSStatusFieldVirtualNetwork = "virtualNetwork"
)

type EKSStatus struct {
	SecurityGroups []string `json:"securityGroups,omitempty" yaml:"securityGroups,omitempty"`
	Subnets        []string `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	VirtualNetwork string   `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
}
