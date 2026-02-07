package client

const (
	MachineAddressType         = "machineAddress"
	MachineAddressFieldAddress = "address"
	MachineAddressFieldType    = "type"
)

type MachineAddress struct {
	Address string `json:"address,omitempty" yaml:"address,omitempty"`
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
}
