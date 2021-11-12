package client

const (
	RotateCertificatesType                = "rotateCertificates"
	RotateCertificatesFieldCACertificates = "caCertificates"
	RotateCertificatesFieldServices       = "services"
)

type RotateCertificates struct {
	CACertificates bool   `json:"caCertificates,omitempty" yaml:"caCertificates,omitempty"`
	Services       string `json:"services,omitempty" yaml:"services,omitempty"`
}
