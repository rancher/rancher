package client

const (
	APIServerConfigType                 = "apiServerConfig"
	APIServerConfigFieldBasicAuth       = "basicAuth"
	APIServerConfigFieldBearerToken     = "bearerToken"
	APIServerConfigFieldBearerTokenFile = "bearerTokenFile"
	APIServerConfigFieldHost            = "host"
	APIServerConfigFieldTLSConfig       = "tlsConfig"
)

type APIServerConfig struct {
	BasicAuth       *BasicAuth `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	BearerToken     string     `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
	BearerTokenFile string     `json:"bearerTokenFile,omitempty" yaml:"bearerTokenFile,omitempty"`
	Host            string     `json:"host,omitempty" yaml:"host,omitempty"`
	TLSConfig       *TLSConfig `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
}
