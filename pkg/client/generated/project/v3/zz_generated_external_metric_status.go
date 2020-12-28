package client

const (
	ExternalMetricStatusType         = "externalMetricStatus"
	ExternalMetricStatusFieldCurrent = "current"
	ExternalMetricStatusFieldMetric  = "metric"
)

type ExternalMetricStatus struct {
	Current *MetricValueStatus `json:"current,omitempty" yaml:"current,omitempty"`
	Metric  *MetricIdentifier  `json:"metric,omitempty" yaml:"metric,omitempty"`
}
