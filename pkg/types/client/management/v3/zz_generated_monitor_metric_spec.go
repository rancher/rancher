package client

const (
	MonitorMetricSpecType              = "monitorMetricSpec"
	MonitorMetricSpecFieldDescription  = "description"
	MonitorMetricSpecFieldExpression   = "expression"
	MonitorMetricSpecFieldLegendFormat = "legendFormat"
)

type MonitorMetricSpec struct {
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Expression   string `json:"expression,omitempty" yaml:"expression,omitempty"`
	LegendFormat string `json:"legendFormat,omitempty" yaml:"legendFormat,omitempty"`
}
