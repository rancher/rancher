package client

const (
	MachineConditionType                    = "machineCondition"
	MachineConditionFieldLastTransitionTime = "lastTransitionTime"
	MachineConditionFieldLastUpdateTime     = "lastUpdateTime"
	MachineConditionFieldReason             = "reason"
	MachineConditionFieldStatus             = "status"
	MachineConditionFieldType               = "type"
)

type MachineCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
