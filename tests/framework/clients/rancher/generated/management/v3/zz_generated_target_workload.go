package client

const (
	TargetWorkloadType                     = "targetWorkload"
	TargetWorkloadFieldAvailablePercentage = "availablePercentage"
	TargetWorkloadFieldSelector            = "selector"
	TargetWorkloadFieldWorkloadID          = "workloadId"
)

type TargetWorkload struct {
	AvailablePercentage int64             `json:"availablePercentage,omitempty" yaml:"availablePercentage,omitempty"`
	Selector            map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`
	WorkloadID          string            `json:"workloadId,omitempty" yaml:"workloadId,omitempty"`
}
