package client

const (
	ConfigMapEnvSourceType          = "configMapEnvSource"
	ConfigMapEnvSourceFieldName     = "name"
	ConfigMapEnvSourceFieldOptional = "optional"
)

type ConfigMapEnvSource struct {
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Optional *bool  `json:"optional,omitempty" yaml:"optional,omitempty"`
}
