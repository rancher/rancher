package client

const (
	PortType          = "port"
	PortFieldName     = "name"
	PortFieldNumber   = "number"
	PortFieldProtocol = "protocol"
)

type Port struct {
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Number   int64  `json:"number,omitempty" yaml:"number,omitempty"`
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}
