package client

const (
	RKETaintType           = "rkeTaint"
	RKETaintFieldEffect    = "effect"
	RKETaintFieldKey       = "key"
	RKETaintFieldTimeAdded = "timeAdded"
	RKETaintFieldValue     = "value"
)

type RKETaint struct {
	Effect    string `json:"effect,omitempty" yaml:"effect,omitempty"`
	Key       string `json:"key,omitempty" yaml:"key,omitempty"`
	TimeAdded string `json:"timeAdded,omitempty" yaml:"timeAdded,omitempty"`
	Value     string `json:"value,omitempty" yaml:"value,omitempty"`
}
