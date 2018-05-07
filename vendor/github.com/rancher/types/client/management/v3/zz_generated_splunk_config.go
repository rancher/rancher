package client

const (
	SplunkConfigType           = "splunkConfig"
	SplunkConfigFieldEndpoint  = "endpoint"
	SplunkConfigFieldSSLVerify = "sslVerify"
	SplunkConfigFieldSource    = "source"
	SplunkConfigFieldToken     = "token"
)

type SplunkConfig struct {
	Endpoint  string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	SSLVerify bool   `json:"sslVerify,omitempty" yaml:"sslVerify,omitempty"`
	Source    string `json:"source,omitempty" yaml:"source,omitempty"`
	Token     string `json:"token,omitempty" yaml:"token,omitempty"`
}
