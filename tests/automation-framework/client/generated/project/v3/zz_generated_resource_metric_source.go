package client

const (
	ResourceMetricSourceType        = "resourceMetricSource"
	ResourceMetricSourceFieldName   = "name"
	ResourceMetricSourceFieldTarget = "target"
)

type ResourceMetricSource struct {
	Name   string        `json:"name,omitempty" yaml:"name,omitempty"`
	Target *MetricTarget `json:"target,omitempty" yaml:"target,omitempty"`
}
