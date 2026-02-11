package client

const (
	MachineV1Beta1DeprecatedStatusType                = "machineV1Beta1DeprecatedStatus"
	MachineV1Beta1DeprecatedStatusFieldConditions     = "conditions"
	MachineV1Beta1DeprecatedStatusFieldFailureMessage = "failureMessage"
	MachineV1Beta1DeprecatedStatusFieldFailureReason  = "failureReason"
)

type MachineV1Beta1DeprecatedStatus struct {
	Conditions     []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	FailureMessage string      `json:"failureMessage,omitempty" yaml:"failureMessage,omitempty"`
	FailureReason  string      `json:"failureReason,omitempty" yaml:"failureReason,omitempty"`
}
