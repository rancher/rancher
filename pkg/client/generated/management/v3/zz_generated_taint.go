package client

const (
	TaintType           = "taint"
	TaintFieldEffect    = "effect"
	TaintFieldKey       = "key"
	TaintFieldTimeAdded = "timeAdded"
	TaintFieldValue     = "value"
)

type Taint struct {
	Effect    string `json:"effect,omitempty" yaml:"effect,omitempty"`
	Key       string `json:"key,omitempty" yaml:"key,omitempty"`
	TimeAdded string `json:"timeAdded,omitempty" yaml:"timeAdded,omitempty"`
	Value     string `json:"value,omitempty" yaml:"value,omitempty"`
}
