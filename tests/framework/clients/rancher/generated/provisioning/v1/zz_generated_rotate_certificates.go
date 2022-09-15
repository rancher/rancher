package client

const (
	RotateCertificatesType            = "rotateCertificates"
	RotateCertificatesFieldGeneration = "generation"
	RotateCertificatesFieldServices   = "services"
)

type RotateCertificates struct {
	Generation int64    `json:"generation,omitempty" yaml:"generation,omitempty"`
	Services   []string `json:"services,omitempty" yaml:"services,omitempty"`
}
