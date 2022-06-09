package client

const (
	SplunkConfigType               = "splunkConfig"
	SplunkConfigFieldCertificate   = "certificate"
	SplunkConfigFieldClientCert    = "clientCert"
	SplunkConfigFieldClientKey     = "clientKey"
	SplunkConfigFieldClientKeyPass = "clientKeyPass"
	SplunkConfigFieldEndpoint      = "endpoint"
	SplunkConfigFieldIndex         = "index"
	SplunkConfigFieldSSLVerify     = "sslVerify"
	SplunkConfigFieldSource        = "source"
	SplunkConfigFieldToken         = "token"
)

type SplunkConfig struct {
	Certificate   string `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ClientCert    string `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey     string `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
	ClientKeyPass string `json:"clientKeyPass,omitempty" yaml:"clientKeyPass,omitempty"`
	Endpoint      string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Index         string `json:"index,omitempty" yaml:"index,omitempty"`
	SSLVerify     bool   `json:"sslVerify,omitempty" yaml:"sslVerify,omitempty"`
	Source        string `json:"source,omitempty" yaml:"source,omitempty"`
	Token         string `json:"token,omitempty" yaml:"token,omitempty"`
}
