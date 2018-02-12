package client

const (
	TargetWorkloadType                     = "targetWorkload"
	TargetWorkloadFieldAvailablePercentage = "availablePercentage"
	TargetWorkloadFieldSelector            = "selector"
	TargetWorkloadFieldType                = "type"
	TargetWorkloadFieldWorkloadID          = "workloadId"
)

type TargetWorkload struct {
	AvailablePercentage *int64            `json:"availablePercentage,omitempty"`
	Selector            map[string]string `json:"selector,omitempty"`
	Type                string            `json:"type,omitempty"`
	WorkloadID          string            `json:"workloadId,omitempty"`
}
