package client

const (
	ObjectMetricSourceType                 = "objectMetricSource"
	ObjectMetricSourceFieldDescribedObject = "describedObject"
	ObjectMetricSourceFieldMetric          = "metric"
	ObjectMetricSourceFieldTarget          = "target"
)

type ObjectMetricSource struct {
	DescribedObject *CrossVersionObjectReference `json:"describedObject,omitempty" yaml:"describedObject,omitempty"`
	Metric          *MetricIdentifier            `json:"metric,omitempty" yaml:"metric,omitempty"`
	Target          *MetricTarget                `json:"target,omitempty" yaml:"target,omitempty"`
}
