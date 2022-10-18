package client

const (
	PortCheckType          = "portCheck"
	PortCheckFieldAddress  = "address"
	PortCheckFieldPort     = "port"
	PortCheckFieldProtocol = "protocol"
)

type PortCheck struct {
	Address  string `json:"address,omitempty" yaml:"address,omitempty"`
	Port     int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}
