package client

const (
	PodsMetricSourceType        = "podsMetricSource"
	PodsMetricSourceFieldMetric = "metric"
	PodsMetricSourceFieldTarget = "target"
)

type PodsMetricSource struct {
	Metric *MetricIdentifier `json:"metric,omitempty" yaml:"metric,omitempty"`
	Target *MetricTarget     `json:"target,omitempty" yaml:"target,omitempty"`
}
