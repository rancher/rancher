package client

const (
	MachineReadinessGateType               = "machineReadinessGate"
	MachineReadinessGateFieldConditionType = "conditionType"
)

type MachineReadinessGate struct {
	ConditionType string `json:"conditionType,omitempty" yaml:"conditionType,omitempty"`
}
