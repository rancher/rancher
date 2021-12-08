package client

const (
	DestinationType        = "destination"
	DestinationFieldHost   = "host"
	DestinationFieldPort   = "port"
	DestinationFieldSubset = "subset"
)

type Destination struct {
	Host   string        `json:"host,omitempty" yaml:"host,omitempty"`
	Port   *PortSelector `json:"port,omitempty" yaml:"port,omitempty"`
	Subset string        `json:"subset,omitempty" yaml:"subset,omitempty"`
}
