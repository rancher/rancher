package client

const (
	MachineDriverStatusType            = "machineDriverStatus"
	MachineDriverStatusFieldConditions = "conditions"
)

type MachineDriverStatus struct {
	Conditions []MachineDriverCondition `json:"conditions,omitempty"`
}
