package client

const (
	PodReadinessGateType               = "podReadinessGate"
	PodReadinessGateFieldConditionType = "conditionType"
)

type PodReadinessGate struct {
	ConditionType string `json:"conditionType,omitempty" yaml:"conditionType,omitempty"`
}
