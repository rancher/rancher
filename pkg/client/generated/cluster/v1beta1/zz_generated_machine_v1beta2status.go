package client

const (
	MachineV1Beta2StatusType            = "machineV1Beta2Status"
	MachineV1Beta2StatusFieldConditions = "conditions"
)

type MachineV1Beta2Status struct {
	Conditions []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
