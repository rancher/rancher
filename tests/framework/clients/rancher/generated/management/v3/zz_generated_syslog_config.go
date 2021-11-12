package client

const (
	SyslogConfigType             = "syslogConfig"
	SyslogConfigFieldCertificate = "certificate"
	SyslogConfigFieldClientCert  = "clientCert"
	SyslogConfigFieldClientKey   = "clientKey"
	SyslogConfigFieldEnableTLS   = "enableTls"
	SyslogConfigFieldEndpoint    = "endpoint"
	SyslogConfigFieldProgram     = "program"
	SyslogConfigFieldProtocol    = "protocol"
	SyslogConfigFieldSSLVerify   = "sslVerify"
	SyslogConfigFieldSeverity    = "severity"
	SyslogConfigFieldToken       = "token"
)

type SyslogConfig struct {
	Certificate string `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ClientCert  string `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey   string `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
	EnableTLS   bool   `json:"enableTls,omitempty" yaml:"enableTls,omitempty"`
	Endpoint    string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Program     string `json:"program,omitempty" yaml:"program,omitempty"`
	Protocol    string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	SSLVerify   bool   `json:"sslVerify,omitempty" yaml:"sslVerify,omitempty"`
	Severity    string `json:"severity,omitempty" yaml:"severity,omitempty"`
	Token       string `json:"token,omitempty" yaml:"token,omitempty"`
}
