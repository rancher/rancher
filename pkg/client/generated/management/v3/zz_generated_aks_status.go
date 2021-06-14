package client

const (
	AKSStatusType                       = "aksStatus"
	AKSStatusFieldPrivateRequiresTunnel = "privateRequiresTunnel"
	AKSStatusFieldRBACEnabled           = "rbacEnabled"
	AKSStatusFieldUpstreamSpec          = "upstreamSpec"
)

type AKSStatus struct {
	PrivateRequiresTunnel *bool                 `json:"privateRequiresTunnel,omitempty" yaml:"privateRequiresTunnel,omitempty"`
	RBACEnabled           *bool                 `json:"rbacEnabled,omitempty" yaml:"rbacEnabled,omitempty"`
	UpstreamSpec          *AKSClusterConfigSpec `json:"upstreamSpec,omitempty" yaml:"upstreamSpec,omitempty"`
}
