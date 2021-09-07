package client

const (
	PortStatusType          = "portStatus"
	PortStatusFieldError    = "error"
	PortStatusFieldPort     = "port"
	PortStatusFieldProtocol = "protocol"
)

type PortStatus struct {
	Error    string `json:"error,omitempty" yaml:"error,omitempty"`
	Port     int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}
