package client

const (
	StatefulSetUpdateStrategyType           = "statefulSetUpdateStrategy"
	StatefulSetUpdateStrategyFieldPartition = "partition"
	StatefulSetUpdateStrategyFieldStrategy  = "strategy"
)

type StatefulSetUpdateStrategy struct {
	Partition *int64 `json:"partition,omitempty" yaml:"partition,omitempty"`
	Strategy  string `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
