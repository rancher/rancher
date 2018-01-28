package client

const (
	ElasticsearchConfigType              = "elasticsearchConfig"
	ElasticsearchConfigFieldAuthPassword = "authPassword"
	ElasticsearchConfigFieldAuthUserName = "authUsername"
	ElasticsearchConfigFieldDateFormat   = "dateFormat"
	ElasticsearchConfigFieldHost         = "host"
	ElasticsearchConfigFieldIndexPrefix  = "indexPrefix"
	ElasticsearchConfigFieldPort         = "port"
)

type ElasticsearchConfig struct {
	AuthPassword string `json:"authPassword,omitempty"`
	AuthUserName string `json:"authUsername,omitempty"`
	DateFormat   string `json:"dateFormat,omitempty"`
	Host         string `json:"host,omitempty"`
	IndexPrefix  string `json:"indexPrefix,omitempty"`
	Port         *int64 `json:"port,omitempty"`
}
