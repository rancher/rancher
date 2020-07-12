package client

const (
	RotateCertificateOutputType         = "rotateCertificateOutput"
	RotateCertificateOutputFieldMessage = "message"
)

type RotateCertificateOutput struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}
