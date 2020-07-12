package client

const (
	ExternalMetricSourceType        = "externalMetricSource"
	ExternalMetricSourceFieldMetric = "metric"
	ExternalMetricSourceFieldTarget = "target"
)

type ExternalMetricSource struct {
	Metric *MetricIdentifier `json:"metric,omitempty" yaml:"metric,omitempty"`
	Target *MetricTarget     `json:"target,omitempty" yaml:"target,omitempty"`
}
