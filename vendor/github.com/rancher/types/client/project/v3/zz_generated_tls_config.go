package client

const (
	TLSConfigType                    = "tlsConfig"
	TLSConfigFieldCAFile             = "caFile"
	TLSConfigFieldCertFile           = "certFile"
	TLSConfigFieldInsecureSkipVerify = "insecureSkipVerify"
	TLSConfigFieldKeyFile            = "keyFile"
	TLSConfigFieldServerName         = "serverName"
)

type TLSConfig struct {
	CAFile             string `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	CertFile           string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	KeyFile            string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	ServerName         string `json:"serverName,omitempty" yaml:"serverName,omitempty"`
}
