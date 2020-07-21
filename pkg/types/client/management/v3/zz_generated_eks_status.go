package client

const (
	EKSStatusType                         = "eksStatus"
	EKSStatusFieldGeneratedSecurityGroups = "generatedSecurityGroups"
	EKSStatusFieldGeneratedServiceRole    = "generatedServiceRole"
	EKSStatusFieldGeneratedSubnets        = "generatedSubnets"
	EKSStatusFieldGeneratedVirtualNetwork = "generatedVirtualNetwork"
)

type EKSStatus struct {
	GeneratedSecurityGroups []string `json:"generatedSecurityGroups,omitempty" yaml:"generatedSecurityGroups,omitempty"`
	GeneratedServiceRole    string   `json:"generatedServiceRole,omitempty" yaml:"generatedServiceRole,omitempty"`
	GeneratedSubnets        []string `json:"generatedSubnets,omitempty" yaml:"generatedSubnets,omitempty"`
	GeneratedVirtualNetwork string   `json:"generatedVirtualNetwork,omitempty" yaml:"generatedVirtualNetwork,omitempty"`
}
