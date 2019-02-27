package client

const (
	PortSelectorType        = "portSelector"
	PortSelectorFieldName   = "name"
	PortSelectorFieldNumber = "number"
)

type PortSelector struct {
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Number int64  `json:"number,omitempty" yaml:"number,omitempty"`
}
