package client

const (
	RuntimeClassStrategyOptionsType                          = "runtimeClassStrategyOptions"
	RuntimeClassStrategyOptionsFieldAllowedRuntimeClassNames = "allowedRuntimeClassNames"
	RuntimeClassStrategyOptionsFieldDefaultRuntimeClassName  = "defaultRuntimeClassName"
)

type RuntimeClassStrategyOptions struct {
	AllowedRuntimeClassNames []string `json:"allowedRuntimeClassNames,omitempty" yaml:"allowedRuntimeClassNames,omitempty"`
	DefaultRuntimeClassName  string   `json:"defaultRuntimeClassName,omitempty" yaml:"defaultRuntimeClassName,omitempty"`
}
