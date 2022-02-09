package client

const (
	RemoteWriteSpecType                     = "remoteWriteSpec"
	RemoteWriteSpecFieldBasicAuth           = "basicAuth"
	RemoteWriteSpecFieldBearerToken         = "bearerToken"
	RemoteWriteSpecFieldBearerTokenFile     = "bearerTokenFile"
	RemoteWriteSpecFieldHeaders             = "headers"
	RemoteWriteSpecFieldMetadataConfig      = "metadataConfig"
	RemoteWriteSpecFieldName                = "name"
	RemoteWriteSpecFieldProxyURL            = "proxyUrl"
	RemoteWriteSpecFieldQueueConfig         = "queueConfig"
	RemoteWriteSpecFieldRemoteTimeout       = "remoteTimeout"
	RemoteWriteSpecFieldTLSConfig           = "tlsConfig"
	RemoteWriteSpecFieldURL                 = "url"
	RemoteWriteSpecFieldWriteRelabelConfigs = "writeRelabelConfigs"
)

type RemoteWriteSpec struct {
	BasicAuth           *BasicAuth        `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	BearerToken         string            `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
	BearerTokenFile     string            `json:"bearerTokenFile,omitempty" yaml:"bearerTokenFile,omitempty"`
	Headers             map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	MetadataConfig      *MetadataConfig   `json:"metadataConfig,omitempty" yaml:"metadataConfig,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	ProxyURL            string            `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	QueueConfig         *QueueConfig      `json:"queueConfig,omitempty" yaml:"queueConfig,omitempty"`
	RemoteTimeout       string            `json:"remoteTimeout,omitempty" yaml:"remoteTimeout,omitempty"`
	TLSConfig           *TLSConfig        `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
	URL                 string            `json:"url,omitempty" yaml:"url,omitempty"`
	WriteRelabelConfigs []RelabelConfig   `json:"writeRelabelConfigs,omitempty" yaml:"writeRelabelConfigs,omitempty"`
}
