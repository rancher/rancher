package client

const (
	MetricNamesOutputType       = "metricNamesOutput"
	MetricNamesOutputFieldNames = "names"
	MetricNamesOutputFieldType  = "type"
)

type MetricNamesOutput struct {
	Names []string `json:"names,omitempty" yaml:"names,omitempty"`
	Type  string   `json:"type,omitempty" yaml:"type,omitempty"`
}
