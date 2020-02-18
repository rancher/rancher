package client

const (
	LinearAutoscalerParamsType                           = "linearAutoscalerParams"
	LinearAutoscalerParamsFieldCoresPerReplica           = "coresPerReplica"
	LinearAutoscalerParamsFieldMax                       = "max"
	LinearAutoscalerParamsFieldMin                       = "min"
	LinearAutoscalerParamsFieldNodesPerReplica           = "nodesPerReplica"
	LinearAutoscalerParamsFieldPreventSinglePointFailure = "preventSinglePointFailure"
)

type LinearAutoscalerParams struct {
	CoresPerReplica           float64 `json:"coresPerReplica,omitempty" yaml:"coresPerReplica,omitempty"`
	Max                       int64   `json:"max,omitempty" yaml:"max,omitempty"`
	Min                       int64   `json:"min,omitempty" yaml:"min,omitempty"`
	NodesPerReplica           float64 `json:"nodesPerReplica,omitempty" yaml:"nodesPerReplica,omitempty"`
	PreventSinglePointFailure bool    `json:"preventSinglePointFailure,omitempty" yaml:"preventSinglePointFailure,omitempty"`
}
