package client

const (
	IngressTLSType               = "ingressTLS"
	IngressTLSFieldCertificateId = "certificateId"
	IngressTLSFieldHosts         = "hosts"
)

type IngressTLS struct {
	CertificateId string   `json:"certificateId,omitempty" yaml:"certificateId,omitempty"`
	Hosts         []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}
