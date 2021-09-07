package client

const (
	MetricTargetType              = "metricTarget"
	MetricTargetFieldAverageValue = "averageValue"
	MetricTargetFieldType         = "type"
	MetricTargetFieldUtilization  = "utilization"
	MetricTargetFieldValue        = "value"
)

type MetricTarget struct {
	AverageValue string `json:"averageValue,omitempty" yaml:"averageValue,omitempty"`
	Type         string `json:"type,omitempty" yaml:"type,omitempty"`
	Utilization  *int64 `json:"utilization,omitempty" yaml:"utilization,omitempty"`
	Value        string `json:"value,omitempty" yaml:"value,omitempty"`
}
