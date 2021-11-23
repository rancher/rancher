package client

const (
	RemoteWriteSpecType                     = "remoteWriteSpec"
	RemoteWriteSpecFieldAuthorization       = "authorization"
	RemoteWriteSpecFieldBasicAuth           = "basicAuth"
	RemoteWriteSpecFieldBearerToken         = "bearerToken"
	RemoteWriteSpecFieldBearerTokenFile     = "bearerTokenFile"
	RemoteWriteSpecFieldHeaders             = "headers"
	RemoteWriteSpecFieldMetadataConfig      = "metadataConfig"
	RemoteWriteSpecFieldName                = "name"
	RemoteWriteSpecFieldOAuth2              = "oauth2"
	RemoteWriteSpecFieldProxyURL            = "proxyUrl"
	RemoteWriteSpecFieldQueueConfig         = "queueConfig"
	RemoteWriteSpecFieldRemoteTimeout       = "remoteTimeout"
	RemoteWriteSpecFieldSendExemplars       = "sendExemplars"
	RemoteWriteSpecFieldSigv4               = "sigv4"
	RemoteWriteSpecFieldTLSConfig           = "tlsConfig"
	RemoteWriteSpecFieldURL                 = "url"
	RemoteWriteSpecFieldWriteRelabelConfigs = "writeRelabelConfigs"
)

type RemoteWriteSpec struct {
	Authorization       *Authorization    `json:"authorization,omitempty" yaml:"authorization,omitempty"`
	BasicAuth           *BasicAuth        `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	BearerToken         string            `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
	BearerTokenFile     string            `json:"bearerTokenFile,omitempty" yaml:"bearerTokenFile,omitempty"`
	Headers             map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	MetadataConfig      *MetadataConfig   `json:"metadataConfig,omitempty" yaml:"metadataConfig,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	OAuth2              *OAuth2           `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`
	ProxyURL            string            `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	QueueConfig         *QueueConfig      `json:"queueConfig,omitempty" yaml:"queueConfig,omitempty"`
	RemoteTimeout       string            `json:"remoteTimeout,omitempty" yaml:"remoteTimeout,omitempty"`
	SendExemplars       *bool             `json:"sendExemplars,omitempty" yaml:"sendExemplars,omitempty"`
	Sigv4               *Sigv4            `json:"sigv4,omitempty" yaml:"sigv4,omitempty"`
	TLSConfig           *TLSConfig        `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
	URL                 string            `json:"url,omitempty" yaml:"url,omitempty"`
	WriteRelabelConfigs []RelabelConfig   `json:"writeRelabelConfigs,omitempty" yaml:"writeRelabelConfigs,omitempty"`
}
