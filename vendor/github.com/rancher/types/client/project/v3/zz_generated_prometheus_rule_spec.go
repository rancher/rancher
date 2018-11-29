package client

const (
	PrometheusRuleSpecType        = "prometheusRuleSpec"
	PrometheusRuleSpecFieldGroups = "groups"
)

type PrometheusRuleSpec struct {
	Groups []RuleGroup `json:"groups,omitempty" yaml:"groups,omitempty"`
}
