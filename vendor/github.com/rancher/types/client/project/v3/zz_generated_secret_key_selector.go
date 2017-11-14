package client

const (
	SecretKeySelectorType          = "secretKeySelector"
	SecretKeySelectorFieldKey      = "key"
	SecretKeySelectorFieldName     = "name"
	SecretKeySelectorFieldOptional = "optional"
)

type SecretKeySelector struct {
	Key      string `json:"key,omitempty"`
	Name     string `json:"name,omitempty"`
	Optional *bool  `json:"optional,omitempty"`
}
