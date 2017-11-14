package client

const (
	ConfigMapKeySelectorType          = "configMapKeySelector"
	ConfigMapKeySelectorFieldKey      = "key"
	ConfigMapKeySelectorFieldName     = "name"
	ConfigMapKeySelectorFieldOptional = "optional"
)

type ConfigMapKeySelector struct {
	Key      string `json:"key,omitempty"`
	Name     string `json:"name,omitempty"`
	Optional *bool  `json:"optional,omitempty"`
}
