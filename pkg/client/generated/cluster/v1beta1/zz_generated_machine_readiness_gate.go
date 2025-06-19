package client

const (
	MachineReadinessGateType               = "machineReadinessGate"
	MachineReadinessGateFieldConditionType = "conditionType"
	MachineReadinessGateFieldPolarity      = "polarity"
)

type MachineReadinessGate struct {
	ConditionType string `json:"conditionType,omitempty" yaml:"conditionType,omitempty"`
	Polarity      string `json:"polarity,omitempty" yaml:"polarity,omitempty"`
}
