package client

const (
	VirtualServiceSpecType          = "virtualServiceSpec"
	VirtualServiceSpecFieldGateways = "gateways"
	VirtualServiceSpecFieldHTTP     = "http"
	VirtualServiceSpecFieldHosts    = "hosts"
	VirtualServiceSpecFieldTCP      = "tcp"
	VirtualServiceSpecFieldTLS      = "tls"
)

type VirtualServiceSpec struct {
	Gateways []string    `json:"gateways,omitempty" yaml:"gateways,omitempty"`
	HTTP     []HTTPRoute `json:"http,omitempty" yaml:"http,omitempty"`
	Hosts    []string    `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	TCP      []TCPRoute  `json:"tcp,omitempty" yaml:"tcp,omitempty"`
	TLS      []TLSRoute  `json:"tls,omitempty" yaml:"tls,omitempty"`
}
