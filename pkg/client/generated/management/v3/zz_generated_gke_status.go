package client

const (
	GKEStatusType              = "gkeStatus"
	GKEStatusFieldUpstreamSpec = "upstreamSpec"
)

type GKEStatus struct {
	UpstreamSpec *GKEClusterConfigSpec `json:"upstreamSpec,omitempty" yaml:"upstreamSpec,omitempty"`
}
