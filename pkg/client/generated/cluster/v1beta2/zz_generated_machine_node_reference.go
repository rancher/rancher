package client

const (
	MachineNodeReferenceType      = "machineNodeReference"
	MachineNodeReferenceFieldName = "name"
)

type MachineNodeReference struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
