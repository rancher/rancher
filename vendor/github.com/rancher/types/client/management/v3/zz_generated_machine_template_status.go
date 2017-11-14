package client

const (
	MachineTemplateStatusType            = "machineTemplateStatus"
	MachineTemplateStatusFieldConditions = "conditions"
)

type MachineTemplateStatus struct {
	Conditions []MachineTemplateCondition `json:"conditions,omitempty"`
}
