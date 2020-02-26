package client

const (
	TLSConfigType                    = "tlsConfig"
	TLSConfigFieldCA                 = "ca"
	TLSConfigFieldCAFile             = "caFile"
	TLSConfigFieldCert               = "cert"
	TLSConfigFieldCertFile           = "certFile"
	TLSConfigFieldInsecureSkipVerify = "insecureSkipVerify"
	TLSConfigFieldKeyFile            = "keyFile"
	TLSConfigFieldKeySecret          = "keySecret"
	TLSConfigFieldServerName         = "serverName"
)

type TLSConfig struct {
	CA                 *SecretOrConfigMap `json:"ca,omitempty" yaml:"ca,omitempty"`
	CAFile             string             `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	Cert               *SecretOrConfigMap `json:"cert,omitempty" yaml:"cert,omitempty"`
	CertFile           string             `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	InsecureSkipVerify bool               `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	KeyFile            string             `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	KeySecret          *SecretKeySelector `json:"keySecret,omitempty" yaml:"keySecret,omitempty"`
	ServerName         string             `json:"serverName,omitempty" yaml:"serverName,omitempty"`
}
