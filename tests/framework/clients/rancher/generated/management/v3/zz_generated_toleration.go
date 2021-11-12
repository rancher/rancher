package client

const (
	TolerationType                   = "toleration"
	TolerationFieldEffect            = "effect"
	TolerationFieldKey               = "key"
	TolerationFieldOperator          = "operator"
	TolerationFieldTolerationSeconds = "tolerationSeconds"
	TolerationFieldValue             = "value"
)

type Toleration struct {
	Effect            string `json:"effect,omitempty" yaml:"effect,omitempty"`
	Key               string `json:"key,omitempty" yaml:"key,omitempty"`
	Operator          string `json:"operator,omitempty" yaml:"operator,omitempty"`
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty" yaml:"tolerationSeconds,omitempty"`
	Value             string `json:"value,omitempty" yaml:"value,omitempty"`
}
