package client

const (
	ElasticsearchConfigType              = "elasticsearchConfig"
	ElasticsearchConfigFieldAuthPassword = "authPassword"
	ElasticsearchConfigFieldAuthUserName = "authUsername"
	ElasticsearchConfigFieldDateFormat   = "dateFormat"
	ElasticsearchConfigFieldEndpoint     = "endpoint"
	ElasticsearchConfigFieldIndexPrefix  = "indexPrefix"
)

type ElasticsearchConfig struct {
	AuthPassword string `json:"authPassword,omitempty"`
	AuthUserName string `json:"authUsername,omitempty"`
	DateFormat   string `json:"dateFormat,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
	IndexPrefix  string `json:"indexPrefix,omitempty"`
}
