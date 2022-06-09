package client

const (
	APIServerConfigType                 = "apiServerConfig"
	APIServerConfigFieldAuthorization   = "authorization"
	APIServerConfigFieldBasicAuth       = "basicAuth"
	APIServerConfigFieldBearerToken     = "bearerToken"
	APIServerConfigFieldBearerTokenFile = "bearerTokenFile"
	APIServerConfigFieldHost            = "host"
	APIServerConfigFieldTLSConfig       = "tlsConfig"
)

type APIServerConfig struct {
	Authorization   *Authorization `json:"authorization,omitempty" yaml:"authorization,omitempty"`
	BasicAuth       *BasicAuth     `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	BearerToken     string         `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
	BearerTokenFile string         `json:"bearerTokenFile,omitempty" yaml:"bearerTokenFile,omitempty"`
	Host            string         `json:"host,omitempty" yaml:"host,omitempty"`
	TLSConfig       *TLSConfig     `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
}
