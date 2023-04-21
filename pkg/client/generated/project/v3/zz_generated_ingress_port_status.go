package client

const (
	IngressPortStatusType          = "ingressPortStatus"
	IngressPortStatusFieldError    = "error"
	IngressPortStatusFieldPort     = "port"
	IngressPortStatusFieldProtocol = "protocol"
)

type IngressPortStatus struct {
	Error    string `json:"error,omitempty" yaml:"error,omitempty"`
	Port     int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}
