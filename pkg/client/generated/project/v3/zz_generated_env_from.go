package client

const (
	EnvFromType            = "envFrom"
	EnvFromFieldSourceKey  = "sourceKey"
	EnvFromFieldSourceName = "sourceName"
	EnvFromFieldTargetKey  = "targetKey"
)

type EnvFrom struct {
	SourceKey  string `json:"sourceKey,omitempty" yaml:"sourceKey,omitempty"`
	SourceName string `json:"sourceName,omitempty" yaml:"sourceName,omitempty"`
	TargetKey  string `json:"targetKey,omitempty" yaml:"targetKey,omitempty"`
}
