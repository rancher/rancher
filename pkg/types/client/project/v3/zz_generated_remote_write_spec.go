package client

const (
	RemoteWriteSpecType                     = "remoteWriteSpec"
	RemoteWriteSpecFieldBasicAuth           = "basicAuth"
	RemoteWriteSpecFieldBearerToken         = "bearerToken"
	RemoteWriteSpecFieldBearerTokenFile     = "bearerTokenFile"
	RemoteWriteSpecFieldProxyURL            = "proxyUrl"
	RemoteWriteSpecFieldQueueConfig         = "queueConfig"
	RemoteWriteSpecFieldRemoteTimeout       = "remoteTimeout"
	RemoteWriteSpecFieldTLSConfig           = "tlsConfig"
	RemoteWriteSpecFieldURL                 = "url"
	RemoteWriteSpecFieldWriteRelabelConfigs = "writeRelabelConfigs"
)

type RemoteWriteSpec struct {
	BasicAuth           *BasicAuth      `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	BearerToken         string          `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
	BearerTokenFile     string          `json:"bearerTokenFile,omitempty" yaml:"bearerTokenFile,omitempty"`
	ProxyURL            string          `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	QueueConfig         *QueueConfig    `json:"queueConfig,omitempty" yaml:"queueConfig,omitempty"`
	RemoteTimeout       string          `json:"remoteTimeout,omitempty" yaml:"remoteTimeout,omitempty"`
	TLSConfig           *TLSConfig      `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
	URL                 string          `json:"url,omitempty" yaml:"url,omitempty"`
	WriteRelabelConfigs []RelabelConfig `json:"writeRelabelConfigs,omitempty" yaml:"writeRelabelConfigs,omitempty"`
}
