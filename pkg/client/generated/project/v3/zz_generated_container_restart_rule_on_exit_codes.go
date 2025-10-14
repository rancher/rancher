package client

const (
	ContainerRestartRuleOnExitCodesType          = "containerRestartRuleOnExitCodes"
	ContainerRestartRuleOnExitCodesFieldOperator = "operator"
	ContainerRestartRuleOnExitCodesFieldValues   = "values"
)

type ContainerRestartRuleOnExitCodes struct {
	Operator string  `json:"operator,omitempty" yaml:"operator,omitempty"`
	Values   []int64 `json:"values,omitempty" yaml:"values,omitempty"`
}
