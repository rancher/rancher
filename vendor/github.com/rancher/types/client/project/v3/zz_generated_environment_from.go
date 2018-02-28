package client

const (
	EnvironmentFromType            = "environmentFrom"
	EnvironmentFromFieldOptional   = "optional"
	EnvironmentFromFieldPrefix     = "prefix"
	EnvironmentFromFieldSource     = "source"
	EnvironmentFromFieldSourceKey  = "sourceKey"
	EnvironmentFromFieldSourceName = "sourceName"
	EnvironmentFromFieldTargetKey  = "targetKey"
)

type EnvironmentFrom struct {
	Optional   bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
	Prefix     string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Source     string `json:"source,omitempty" yaml:"source,omitempty"`
	SourceKey  string `json:"sourceKey,omitempty" yaml:"sourceKey,omitempty"`
	SourceName string `json:"sourceName,omitempty" yaml:"sourceName,omitempty"`
	TargetKey  string `json:"targetKey,omitempty" yaml:"targetKey,omitempty"`
}
