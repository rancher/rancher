package client

const (
	AESConfigurationType      = "aesConfiguration"
	AESConfigurationFieldKeys = "keys"
)

type AESConfiguration struct {
	Keys []Key `json:"keys,omitempty" yaml:"keys,omitempty"`
}
