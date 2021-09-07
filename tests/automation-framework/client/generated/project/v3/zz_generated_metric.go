package client

const (
	MetricType                   = "metric"
	MetricFieldContainerResource = "containerResource"
	MetricFieldCurrent           = "current"
	MetricFieldDescribedObject   = "describedObject"
	MetricFieldName              = "name"
	MetricFieldSelector          = "selector"
	MetricFieldTarget            = "target"
	MetricFieldType              = "type"
)

type Metric struct {
	ContainerResource *ContainerResourceMetricSource `json:"containerResource,omitempty" yaml:"containerResource,omitempty"`
	Current           *MetricValueStatus             `json:"current,omitempty" yaml:"current,omitempty"`
	DescribedObject   *CrossVersionObjectReference   `json:"describedObject,omitempty" yaml:"describedObject,omitempty"`
	Name              string                         `json:"name,omitempty" yaml:"name,omitempty"`
	Selector          *LabelSelector                 `json:"selector,omitempty" yaml:"selector,omitempty"`
	Target            *MetricTarget                  `json:"target,omitempty" yaml:"target,omitempty"`
	Type              string                         `json:"type,omitempty" yaml:"type,omitempty"`
}
