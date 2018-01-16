package client

const (
	MachineDriverConditionType                    = "machineDriverCondition"
	MachineDriverConditionFieldLastTransitionTime = "lastTransitionTime"
	MachineDriverConditionFieldLastUpdateTime     = "lastUpdateTime"
	MachineDriverConditionFieldMessage            = "message"
	MachineDriverConditionFieldReason             = "reason"
	MachineDriverConditionFieldStatus             = "status"
	MachineDriverConditionFieldType               = "type"
)

type MachineDriverCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
