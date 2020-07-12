package client

const (
	WorkloadMetricType        = "workloadMetric"
	WorkloadMetricFieldPath   = "path"
	WorkloadMetricFieldPort   = "port"
	WorkloadMetricFieldSchema = "schema"
)

type WorkloadMetric struct {
	Path   string `json:"path,omitempty" yaml:"path,omitempty"`
	Port   int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Schema string `json:"schema,omitempty" yaml:"schema,omitempty"`
}
