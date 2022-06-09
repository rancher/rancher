package client

const (
	ContainerResourceMetricSourceType           = "containerResourceMetricSource"
	ContainerResourceMetricSourceFieldContainer = "container"
	ContainerResourceMetricSourceFieldName      = "name"
	ContainerResourceMetricSourceFieldTarget    = "target"
)

type ContainerResourceMetricSource struct {
	Container string        `json:"container,omitempty" yaml:"container,omitempty"`
	Name      string        `json:"name,omitempty" yaml:"name,omitempty"`
	Target    *MetricTarget `json:"target,omitempty" yaml:"target,omitempty"`
}
