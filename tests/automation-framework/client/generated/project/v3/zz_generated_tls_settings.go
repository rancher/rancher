package client

const (
	TLSSettingsType                   = "tlsSettings"
	TLSSettingsFieldCaCertificates    = "caCertificates"
	TLSSettingsFieldClientCertificate = "clientCertificate"
	TLSSettingsFieldMode              = "mode"
	TLSSettingsFieldPrivateKey        = "privateKey"
	TLSSettingsFieldSni               = "sni"
	TLSSettingsFieldSubjectAltNames   = "subjectAltNames"
)

type TLSSettings struct {
	CaCertificates    string   `json:"caCertificates,omitempty" yaml:"caCertificates,omitempty"`
	ClientCertificate string   `json:"clientCertificate,omitempty" yaml:"clientCertificate,omitempty"`
	Mode              string   `json:"mode,omitempty" yaml:"mode,omitempty"`
	PrivateKey        string   `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`
	Sni               string   `json:"sni,omitempty" yaml:"sni,omitempty"`
	SubjectAltNames   []string `json:"subjectAltNames,omitempty" yaml:"subjectAltNames,omitempty"`
}
