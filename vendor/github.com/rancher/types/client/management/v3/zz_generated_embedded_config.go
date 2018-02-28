package client

const (
	EmbeddedConfigType                       = "embeddedConfig"
	EmbeddedConfigFieldDateFormat            = "dateFormat"
	EmbeddedConfigFieldElasticsearchEndpoint = "elasticsearchEndpoint"
	EmbeddedConfigFieldIndexPrefix           = "indexPrefix"
	EmbeddedConfigFieldKibanaEndpoint        = "kibanaEndpoint"
)

type EmbeddedConfig struct {
	DateFormat            string `json:"dateFormat,omitempty" yaml:"dateFormat,omitempty"`
	ElasticsearchEndpoint string `json:"elasticsearchEndpoint,omitempty" yaml:"elasticsearchEndpoint,omitempty"`
	IndexPrefix           string `json:"indexPrefix,omitempty" yaml:"indexPrefix,omitempty"`
	KibanaEndpoint        string `json:"kibanaEndpoint,omitempty" yaml:"kibanaEndpoint,omitempty"`
}
