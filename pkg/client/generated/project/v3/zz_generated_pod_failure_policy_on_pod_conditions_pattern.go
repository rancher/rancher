package client

const (
	PodFailurePolicyOnPodConditionsPatternType        = "podFailurePolicyOnPodConditionsPattern"
	PodFailurePolicyOnPodConditionsPatternFieldStatus = "status"
	PodFailurePolicyOnPodConditionsPatternFieldType   = "type"
)

type PodFailurePolicyOnPodConditionsPattern struct {
	Status string `json:"status,omitempty" yaml:"status,omitempty"`
	Type   string `json:"type,omitempty" yaml:"type,omitempty"`
}
