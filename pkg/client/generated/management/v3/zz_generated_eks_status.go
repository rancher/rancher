package client

const (
	EKSStatusType                               = "eksStatus"
	EKSStatusFieldGeneratedNodeRole             = "generatedNodeRole"
	EKSStatusFieldManagedLaunchTemplateID       = "managedLaunchTemplateID"
	EKSStatusFieldManagedLaunchTemplateVersions = "managedLaunchTemplateVersions"
	EKSStatusFieldPrivateRequiresTunnel         = "privateRequiresTunnel"
	EKSStatusFieldSecurityGroups                = "securityGroups"
	EKSStatusFieldSubnets                       = "subnets"
	EKSStatusFieldUpstreamSpec                  = "upstreamSpec"
	EKSStatusFieldVirtualNetwork                = "virtualNetwork"
)

type EKSStatus struct {
	GeneratedNodeRole             string                `json:"generatedNodeRole,omitempty" yaml:"generatedNodeRole,omitempty"`
	ManagedLaunchTemplateID       string                `json:"managedLaunchTemplateID,omitempty" yaml:"managedLaunchTemplateID,omitempty"`
	ManagedLaunchTemplateVersions map[string]string     `json:"managedLaunchTemplateVersions,omitempty" yaml:"managedLaunchTemplateVersions,omitempty"`
	PrivateRequiresTunnel         *bool                 `json:"privateRequiresTunnel,omitempty" yaml:"privateRequiresTunnel,omitempty"`
	SecurityGroups                []string              `json:"securityGroups,omitempty" yaml:"securityGroups,omitempty"`
	Subnets                       []string              `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	UpstreamSpec                  *EKSClusterConfigSpec `json:"upstreamSpec,omitempty" yaml:"upstreamSpec,omitempty"`
	VirtualNetwork                string                `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
}
