package client

const (
	SecretKeySelectorType          = "secretKeySelector"
	SecretKeySelectorFieldKey      = "key"
	SecretKeySelectorFieldName     = "name"
	SecretKeySelectorFieldOptional = "optional"
)

type SecretKeySelector struct {
	Key      string `json:"key,omitempty" yaml:"key,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Optional *bool  `json:"optional,omitempty" yaml:"optional,omitempty"`
}
