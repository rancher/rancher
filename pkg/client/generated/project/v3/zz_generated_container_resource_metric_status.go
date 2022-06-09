package client

const (
	ContainerResourceMetricStatusType           = "containerResourceMetricStatus"
	ContainerResourceMetricStatusFieldContainer = "container"
	ContainerResourceMetricStatusFieldCurrent   = "current"
	ContainerResourceMetricStatusFieldName      = "name"
)

type ContainerResourceMetricStatus struct {
	Container string             `json:"container,omitempty" yaml:"container,omitempty"`
	Current   *MetricValueStatus `json:"current,omitempty" yaml:"current,omitempty"`
	Name      string             `json:"name,omitempty" yaml:"name,omitempty"`
}
