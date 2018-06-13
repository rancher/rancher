package client

const (
	ElasticsearchConfigType               = "elasticsearchConfig"
	ElasticsearchConfigFieldAuthPassword  = "authPassword"
	ElasticsearchConfigFieldAuthUserName  = "authUsername"
	ElasticsearchConfigFieldCertificate   = "certificate"
	ElasticsearchConfigFieldClientCert    = "clientCert"
	ElasticsearchConfigFieldClientKey     = "clientKey"
	ElasticsearchConfigFieldClientKeyPass = "clientKeyPass"
	ElasticsearchConfigFieldDateFormat    = "dateFormat"
	ElasticsearchConfigFieldEndpoint      = "endpoint"
	ElasticsearchConfigFieldIndexPrefix   = "indexPrefix"
	ElasticsearchConfigFieldSSLVerify     = "sslVerify"
)

type ElasticsearchConfig struct {
	AuthPassword  string `json:"authPassword,omitempty" yaml:"authPassword,omitempty"`
	AuthUserName  string `json:"authUsername,omitempty" yaml:"authUsername,omitempty"`
	Certificate   string `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ClientCert    string `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey     string `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
	ClientKeyPass string `json:"clientKeyPass,omitempty" yaml:"clientKeyPass,omitempty"`
	DateFormat    string `json:"dateFormat,omitempty" yaml:"dateFormat,omitempty"`
	Endpoint      string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	IndexPrefix   string `json:"indexPrefix,omitempty" yaml:"indexPrefix,omitempty"`
	SSLVerify     bool   `json:"sslVerify,omitempty" yaml:"sslVerify,omitempty"`
}
