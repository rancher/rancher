package client

const (
	WebTLSConfigType                          = "webTLSConfig"
	WebTLSConfigFieldCert                     = "cert"
	WebTLSConfigFieldCipherSuites             = "cipherSuites"
	WebTLSConfigFieldClientAuthType           = "clientAuthType"
	WebTLSConfigFieldClientCA                 = "client_ca"
	WebTLSConfigFieldCurvePreferences         = "curvePreferences"
	WebTLSConfigFieldKeySecret                = "keySecret"
	WebTLSConfigFieldMaxVersion               = "maxVersion"
	WebTLSConfigFieldMinVersion               = "minVersion"
	WebTLSConfigFieldPreferServerCipherSuites = "preferServerCipherSuites"
)

type WebTLSConfig struct {
	Cert                     *SecretOrConfigMap `json:"cert,omitempty" yaml:"cert,omitempty"`
	CipherSuites             []string           `json:"cipherSuites,omitempty" yaml:"cipherSuites,omitempty"`
	ClientAuthType           string             `json:"clientAuthType,omitempty" yaml:"clientAuthType,omitempty"`
	ClientCA                 *SecretOrConfigMap `json:"client_ca,omitempty" yaml:"client_ca,omitempty"`
	CurvePreferences         []string           `json:"curvePreferences,omitempty" yaml:"curvePreferences,omitempty"`
	KeySecret                *SecretKeySelector `json:"keySecret,omitempty" yaml:"keySecret,omitempty"`
	MaxVersion               string             `json:"maxVersion,omitempty" yaml:"maxVersion,omitempty"`
	MinVersion               string             `json:"minVersion,omitempty" yaml:"minVersion,omitempty"`
	PreferServerCipherSuites *bool              `json:"preferServerCipherSuites,omitempty" yaml:"preferServerCipherSuites,omitempty"`
}
