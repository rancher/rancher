package client

const (
	ContainerRestartRuleType           = "containerRestartRule"
	ContainerRestartRuleFieldAction    = "action"
	ContainerRestartRuleFieldExitCodes = "exitCodes"
)

type ContainerRestartRule struct {
	Action    string                           `json:"action,omitempty" yaml:"action,omitempty"`
	ExitCodes *ContainerRestartRuleOnExitCodes `json:"exitCodes,omitempty" yaml:"exitCodes,omitempty"`
}
