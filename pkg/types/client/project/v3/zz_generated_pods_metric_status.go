package client

const (
	PodsMetricStatusType         = "podsMetricStatus"
	PodsMetricStatusFieldCurrent = "current"
	PodsMetricStatusFieldMetric  = "metric"
)

type PodsMetricStatus struct {
	Current *MetricValueStatus `json:"current,omitempty" yaml:"current,omitempty"`
	Metric  *MetricIdentifier  `json:"metric,omitempty" yaml:"metric,omitempty"`
}
