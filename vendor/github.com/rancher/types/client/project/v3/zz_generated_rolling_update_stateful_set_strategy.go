package client

const (
	RollingUpdateStatefulSetStrategyType           = "rollingUpdateStatefulSetStrategy"
	RollingUpdateStatefulSetStrategyFieldPartition = "partition"
)

type RollingUpdateStatefulSetStrategy struct {
	Partition *int64 `json:"partition,omitempty"`
}
