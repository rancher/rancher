package client

const (
	MachineDeprecatedStatusType         = "machineDeprecatedStatus"
	MachineDeprecatedStatusFieldV1Beta1 = "v1beta1"
)

type MachineDeprecatedStatus struct {
	V1Beta1 *MachineV1Beta1DeprecatedStatus `json:"v1beta1,omitempty" yaml:"v1beta1,omitempty"`
}
