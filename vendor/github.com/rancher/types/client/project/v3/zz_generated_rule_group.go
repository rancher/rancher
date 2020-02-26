package client

const (
	RuleGroupType                         = "ruleGroup"
	RuleGroupFieldInterval                = "interval"
	RuleGroupFieldName                    = "name"
	RuleGroupFieldPartialResponseStrategy = "partial_response_strategy"
	RuleGroupFieldRules                   = "rules"
)

type RuleGroup struct {
	Interval                string `json:"interval,omitempty" yaml:"interval,omitempty"`
	Name                    string `json:"name,omitempty" yaml:"name,omitempty"`
	PartialResponseStrategy string `json:"partial_response_strategy,omitempty" yaml:"partial_response_strategy,omitempty"`
	Rules                   []Rule `json:"rules,omitempty" yaml:"rules,omitempty"`
}
