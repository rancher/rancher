package client

const (
	ElasticsearchConfigType              = "elasticsearchConfig"
	ElasticsearchConfigFieldAuthPassword = "authPassword"
	ElasticsearchConfigFieldAuthUserName = "authUsername"
	ElasticsearchConfigFieldDateFormat   = "dateFormat"
	ElasticsearchConfigFieldEndpoint     = "endpoint"
	ElasticsearchConfigFieldIndexPrefix  = "indexPrefix"
	ElasticsearchConfigFieldSSLVerify    = "sslVerify"
)

type ElasticsearchConfig struct {
	AuthPassword string `json:"authPassword,omitempty" yaml:"authPassword,omitempty"`
	AuthUserName string `json:"authUsername,omitempty" yaml:"authUsername,omitempty"`
	DateFormat   string `json:"dateFormat,omitempty" yaml:"dateFormat,omitempty"`
	Endpoint     string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	IndexPrefix  string `json:"indexPrefix,omitempty" yaml:"indexPrefix,omitempty"`
	SSLVerify    bool   `json:"sslVerify,omitempty" yaml:"sslVerify,omitempty"`
}
