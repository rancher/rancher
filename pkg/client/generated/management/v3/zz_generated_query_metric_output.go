package client

const (
	QueryMetricOutputType        = "queryMetricOutput"
	QueryMetricOutputFieldSeries = "series"
	QueryMetricOutputFieldType   = "type"
)

type QueryMetricOutput struct {
	Series []string `json:"series,omitempty" yaml:"series,omitempty"`
	Type   string   `json:"type,omitempty" yaml:"type,omitempty"`
}
