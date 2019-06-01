package client

const (
	MetricIdentifierType          = "metricIdentifier"
	MetricIdentifierFieldName     = "name"
	MetricIdentifierFieldSelector = "selector"
)

type MetricIdentifier struct {
	Name     string         `json:"name,omitempty" yaml:"name,omitempty"`
	Selector *LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
}
