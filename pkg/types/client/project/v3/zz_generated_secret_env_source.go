package client

const (
	SecretEnvSourceType          = "secretEnvSource"
	SecretEnvSourceFieldName     = "name"
	SecretEnvSourceFieldOptional = "optional"
)

type SecretEnvSource struct {
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Optional *bool  `json:"optional,omitempty" yaml:"optional,omitempty"`
}
