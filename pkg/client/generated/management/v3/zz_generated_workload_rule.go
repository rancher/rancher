package client

const (
	WorkloadRuleType                     = "workloadRule"
	WorkloadRuleFieldAvailablePercentage = "availablePercentage"
	WorkloadRuleFieldSelector            = "selector"
	WorkloadRuleFieldWorkloadID          = "workloadId"
)

type WorkloadRule struct {
	AvailablePercentage int64             `json:"availablePercentage,omitempty" yaml:"availablePercentage,omitempty"`
	Selector            map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`
	WorkloadID          string            `json:"workloadId,omitempty" yaml:"workloadId,omitempty"`
}
