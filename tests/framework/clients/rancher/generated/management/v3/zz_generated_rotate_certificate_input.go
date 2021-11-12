package client

const (
	RotateCertificateInputType                = "rotateCertificateInput"
	RotateCertificateInputFieldCACertificates = "caCertificates"
	RotateCertificateInputFieldServices       = "services"
)

type RotateCertificateInput struct {
	CACertificates bool   `json:"caCertificates,omitempty" yaml:"caCertificates,omitempty"`
	Services       string `json:"services,omitempty" yaml:"services,omitempty"`
}
