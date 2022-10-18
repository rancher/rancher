package client

const (
	GKENodePoolAutoscalingType              = "gkeNodePoolAutoscaling"
	GKENodePoolAutoscalingFieldEnabled      = "enabled"
	GKENodePoolAutoscalingFieldMaxNodeCount = "maxNodeCount"
	GKENodePoolAutoscalingFieldMinNodeCount = "minNodeCount"
)

type GKENodePoolAutoscaling struct {
	Enabled      bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	MaxNodeCount int64 `json:"maxNodeCount,omitempty" yaml:"maxNodeCount,omitempty"`
	MinNodeCount int64 `json:"minNodeCount,omitempty" yaml:"minNodeCount,omitempty"`
}
