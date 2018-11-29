package client

const (
	RuleGroupType          = "ruleGroup"
	RuleGroupFieldInterval = "interval"
	RuleGroupFieldName     = "name"
	RuleGroupFieldRules    = "rules"
)

type RuleGroup struct {
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Rules    []Rule `json:"rules,omitempty" yaml:"rules,omitempty"`
}
