package client

const (
	MachineTaintType             = "machineTaint"
	MachineTaintFieldEffect      = "effect"
	MachineTaintFieldKey         = "key"
	MachineTaintFieldPropagation = "propagation"
	MachineTaintFieldValue       = "value"
)

type MachineTaint struct {
	Effect      string `json:"effect,omitempty" yaml:"effect,omitempty"`
	Key         string `json:"key,omitempty" yaml:"key,omitempty"`
	Propagation string `json:"propagation,omitempty" yaml:"propagation,omitempty"`
	Value       string `json:"value,omitempty" yaml:"value,omitempty"`
}
