package client

const (
	PodSchedulingGateType      = "podSchedulingGate"
	PodSchedulingGateFieldName = "name"
)

type PodSchedulingGate struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
