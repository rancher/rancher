package client

const (
	StatefulSetUpdateStrategyType               = "statefulSetUpdateStrategy"
	StatefulSetUpdateStrategyFieldRollingUpdate = "rollingUpdate"
	StatefulSetUpdateStrategyFieldType          = "type"
)

type StatefulSetUpdateStrategy struct {
	RollingUpdate *RollingUpdateStatefulSetStrategy `json:"rollingUpdate,omitempty"`
	Type          string                            `json:"type,omitempty"`
}
