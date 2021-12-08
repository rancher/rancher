package client

const (
	MetricStatusType                   = "metricStatus"
	MetricStatusFieldContainerResource = "containerResource"
	MetricStatusFieldCurrent           = "current"
	MetricStatusFieldExternal          = "external"
	MetricStatusFieldObject            = "object"
	MetricStatusFieldPods              = "pods"
	MetricStatusFieldResource          = "resource"
	MetricStatusFieldType              = "type"
)

type MetricStatus struct {
	ContainerResource *ContainerResourceMetricStatus `json:"containerResource,omitempty" yaml:"containerResource,omitempty"`
	Current           *MetricValueStatus             `json:"current,omitempty" yaml:"current,omitempty"`
	External          *ExternalMetricStatus          `json:"external,omitempty" yaml:"external,omitempty"`
	Object            *ObjectMetricStatus            `json:"object,omitempty" yaml:"object,omitempty"`
	Pods              *PodsMetricStatus              `json:"pods,omitempty" yaml:"pods,omitempty"`
	Resource          *ResourceMetricStatus          `json:"resource,omitempty" yaml:"resource,omitempty"`
	Type              string                         `json:"type,omitempty" yaml:"type,omitempty"`
}
