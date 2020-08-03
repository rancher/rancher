package client

const (
	ResourceMetricStatusType         = "resourceMetricStatus"
	ResourceMetricStatusFieldCurrent = "current"
	ResourceMetricStatusFieldName    = "name"
)

type ResourceMetricStatus struct {
	Current *MetricValueStatus `json:"current,omitempty" yaml:"current,omitempty"`
	Name    string             `json:"name,omitempty" yaml:"name,omitempty"`
}
