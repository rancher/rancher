package client

const (
	MachineTemplateConditionType                    = "machineTemplateCondition"
	MachineTemplateConditionFieldLastTransitionTime = "lastTransitionTime"
	MachineTemplateConditionFieldLastUpdateTime     = "lastUpdateTime"
	MachineTemplateConditionFieldReason             = "reason"
	MachineTemplateConditionFieldStatus             = "status"
	MachineTemplateConditionFieldType               = "type"
)

type MachineTemplateCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
