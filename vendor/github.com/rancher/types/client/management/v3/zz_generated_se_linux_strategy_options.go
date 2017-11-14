package client

const (
	SELinuxStrategyOptionsType                = "seLinuxStrategyOptions"
	SELinuxStrategyOptionsFieldRule           = "rule"
	SELinuxStrategyOptionsFieldSELinuxOptions = "seLinuxOptions"
)

type SELinuxStrategyOptions struct {
	Rule           string          `json:"rule,omitempty"`
	SELinuxOptions *SELinuxOptions `json:"seLinuxOptions,omitempty"`
}
