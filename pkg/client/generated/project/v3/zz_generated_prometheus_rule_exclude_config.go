package client

const (
	PrometheusRuleExcludeConfigType               = "prometheusRuleExcludeConfig"
	PrometheusRuleExcludeConfigFieldRuleName      = "ruleName"
	PrometheusRuleExcludeConfigFieldRuleNamespace = "ruleNamespace"
)

type PrometheusRuleExcludeConfig struct {
	RuleName      string `json:"ruleName,omitempty" yaml:"ruleName,omitempty"`
	RuleNamespace string `json:"ruleNamespace,omitempty" yaml:"ruleNamespace,omitempty"`
}
