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
	Effect            string `json:"effect,omitempty"`
	Key               string `json:"key,omitempty"`
	Operator          string `json:"operator,omitempty"`
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
	Value             string `json:"value,omitempty"`
}
