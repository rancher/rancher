package client

const (
	GKENodeTaintConfigType        = "gkeNodeTaintConfig"
	GKENodeTaintConfigFieldEffect = "effect"
	GKENodeTaintConfigFieldKey    = "key"
	GKENodeTaintConfigFieldValue  = "value"
)

type GKENodeTaintConfig struct {
	Effect string `json:"effect,omitempty" yaml:"effect,omitempty"`
	Key    string `json:"key,omitempty" yaml:"key,omitempty"`
	Value  string `json:"value,omitempty" yaml:"value,omitempty"`
}
