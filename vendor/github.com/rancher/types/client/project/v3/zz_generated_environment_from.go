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
	Optional   bool   `json:"optional,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	Source     string `json:"source,omitempty"`
	SourceKey  string `json:"sourceKey,omitempty"`
	SourceName string `json:"sourceName,omitempty"`
	TargetKey  string `json:"targetKey,omitempty"`
}
