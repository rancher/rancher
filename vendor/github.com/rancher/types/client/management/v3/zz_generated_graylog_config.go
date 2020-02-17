package client

const (
	GraylogConfigType             = "graylogConfig"
	GraylogConfigFieldCertificate = "certificate"
	GraylogConfigFieldClientCert  = "clientCert"
	GraylogConfigFieldClientKey   = "clientKey"
	GraylogConfigFieldEnableTLS   = "enableTls"
	GraylogConfigFieldEndpoint    = "endpoint"
	GraylogConfigFieldProtocol    = "protocol"
	GraylogConfigFieldSSLVerify   = "sslVerify"
)

type GraylogConfig struct {
	Certificate string `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ClientCert  string `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey   string `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
	EnableTLS   bool   `json:"enableTls,omitempty" yaml:"enableTls,omitempty"`
	Endpoint    string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Protocol    string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	SSLVerify   bool   `json:"sslVerify,omitempty" yaml:"sslVerify,omitempty"`
}
