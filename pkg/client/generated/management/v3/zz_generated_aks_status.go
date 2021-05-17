package client

const (
	AKSStatusType                       = "aksStatus"
	AKSStatusFieldPrivateRequiresTunnel = "privateRequiresTunnel"
	AKSStatusFieldUpstreamSpec          = "upstreamSpec"
)

type AKSStatus struct {
	PrivateRequiresTunnel *bool                 `json:"privateRequiresTunnel,omitempty" yaml:"privateRequiresTunnel,omitempty"`
	UpstreamSpec          *AKSClusterConfigSpec `json:"upstreamSpec,omitempty" yaml:"upstreamSpec,omitempty"`
}
