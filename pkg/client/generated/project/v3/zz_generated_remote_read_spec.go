package client

const (
	RemoteReadSpecType                  = "remoteReadSpec"
	RemoteReadSpecFieldAuthorization    = "authorization"
	RemoteReadSpecFieldBasicAuth        = "basicAuth"
	RemoteReadSpecFieldBearerToken      = "bearerToken"
	RemoteReadSpecFieldBearerTokenFile  = "bearerTokenFile"
	RemoteReadSpecFieldName             = "name"
	RemoteReadSpecFieldOAuth2           = "oauth2"
	RemoteReadSpecFieldProxyURL         = "proxyUrl"
	RemoteReadSpecFieldReadRecent       = "readRecent"
	RemoteReadSpecFieldRemoteTimeout    = "remoteTimeout"
	RemoteReadSpecFieldRequiredMatchers = "requiredMatchers"
	RemoteReadSpecFieldTLSConfig        = "tlsConfig"
	RemoteReadSpecFieldURL              = "url"
)

type RemoteReadSpec struct {
	Authorization    *Authorization    `json:"authorization,omitempty" yaml:"authorization,omitempty"`
	BasicAuth        *BasicAuth        `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	BearerToken      string            `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
	BearerTokenFile  string            `json:"bearerTokenFile,omitempty" yaml:"bearerTokenFile,omitempty"`
	Name             string            `json:"name,omitempty" yaml:"name,omitempty"`
	OAuth2           *OAuth2           `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`
	ProxyURL         string            `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	ReadRecent       bool              `json:"readRecent,omitempty" yaml:"readRecent,omitempty"`
	RemoteTimeout    string            `json:"remoteTimeout,omitempty" yaml:"remoteTimeout,omitempty"`
	RequiredMatchers map[string]string `json:"requiredMatchers,omitempty" yaml:"requiredMatchers,omitempty"`
	TLSConfig        *TLSConfig        `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
	URL              string            `json:"url,omitempty" yaml:"url,omitempty"`
}
