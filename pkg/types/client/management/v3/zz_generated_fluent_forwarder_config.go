package client

const (
	FluentForwarderConfigType               = "fluentForwarderConfig"
	FluentForwarderConfigFieldCertificate   = "certificate"
	FluentForwarderConfigFieldClientCert    = "clientCert"
	FluentForwarderConfigFieldClientKey     = "clientKey"
	FluentForwarderConfigFieldClientKeyPass = "clientKeyPass"
	FluentForwarderConfigFieldCompress      = "compress"
	FluentForwarderConfigFieldEnableTLS     = "enableTls"
	FluentForwarderConfigFieldFluentServers = "fluentServers"
	FluentForwarderConfigFieldSSLVerify     = "sslVerify"
)

type FluentForwarderConfig struct {
	Certificate   string         `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ClientCert    string         `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey     string         `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
	ClientKeyPass string         `json:"clientKeyPass,omitempty" yaml:"clientKeyPass,omitempty"`
	Compress      *bool          `json:"compress,omitempty" yaml:"compress,omitempty"`
	EnableTLS     bool           `json:"enableTls,omitempty" yaml:"enableTls,omitempty"`
	FluentServers []FluentServer `json:"fluentServers,omitempty" yaml:"fluentServers,omitempty"`
	SSLVerify     bool           `json:"sslVerify,omitempty" yaml:"sslVerify,omitempty"`
}
