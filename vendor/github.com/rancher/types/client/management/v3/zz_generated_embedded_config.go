package client

const (
	EmbeddedConfigType                       = "embeddedConfig"
	EmbeddedConfigFieldDateFormat            = "dateFormat"
	EmbeddedConfigFieldElasticsearchEndpoint = "elasticsearchEndpoint"
	EmbeddedConfigFieldIndexPrefix           = "indexPrefix"
	EmbeddedConfigFieldKibanaEndpoint        = "kibanaEndpoint"
	EmbeddedConfigFieldLimitsCPU             = "limitsCpu"
	EmbeddedConfigFieldLimitsMemery          = "limitsMemory"
	EmbeddedConfigFieldRequestsCPU           = "requestsCpu"
	EmbeddedConfigFieldRequestsMemery        = "requestsMemory"
)

type EmbeddedConfig struct {
	DateFormat            string `json:"dateFormat,omitempty" yaml:"dateFormat,omitempty"`
	ElasticsearchEndpoint string `json:"elasticsearchEndpoint,omitempty" yaml:"elasticsearchEndpoint,omitempty"`
	IndexPrefix           string `json:"indexPrefix,omitempty" yaml:"indexPrefix,omitempty"`
	KibanaEndpoint        string `json:"kibanaEndpoint,omitempty" yaml:"kibanaEndpoint,omitempty"`
	LimitsCPU             int64  `json:"limitsCpu,omitempty" yaml:"limitsCpu,omitempty"`
	LimitsMemery          int64  `json:"limitsMemory,omitempty" yaml:"limitsMemory,omitempty"`
	RequestsCPU           int64  `json:"requestsCpu,omitempty" yaml:"requestsCpu,omitempty"`
	RequestsMemery        int64  `json:"requestsMemory,omitempty" yaml:"requestsMemory,omitempty"`
}
