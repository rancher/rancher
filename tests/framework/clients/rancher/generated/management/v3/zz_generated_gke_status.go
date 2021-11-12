package client

const (
	GKEStatusType                       = "gkeStatus"
	GKEStatusFieldPrivateRequiresTunnel = "privateRequiresTunnel"
	GKEStatusFieldUpstreamSpec          = "upstreamSpec"
)

type GKEStatus struct {
	PrivateRequiresTunnel *bool                 `json:"privateRequiresTunnel,omitempty" yaml:"privateRequiresTunnel,omitempty"`
	UpstreamSpec          *GKEClusterConfigSpec `json:"upstreamSpec,omitempty" yaml:"upstreamSpec,omitempty"`
}
