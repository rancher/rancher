package client

const (
	AliStatusType                       = "aliStatus"
	AliStatusFieldPrivateRequiresTunnel = "privateRequiresTunnel"
	AliStatusFieldUpstreamSpec          = "upstreamSpec"
)

type AliStatus struct {
	PrivateRequiresTunnel *bool                 `json:"privateRequiresTunnel,omitempty" yaml:"privateRequiresTunnel,omitempty"`
	UpstreamSpec          *AliClusterConfigSpec `json:"upstreamSpec,omitempty" yaml:"upstreamSpec,omitempty"`
}
