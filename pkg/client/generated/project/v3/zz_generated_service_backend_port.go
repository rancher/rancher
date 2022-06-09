package client

const (
	ServiceBackendPortType        = "serviceBackendPort"
	ServiceBackendPortFieldName   = "name"
	ServiceBackendPortFieldNumber = "number"
)

type ServiceBackendPort struct {
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Number int64  `json:"number,omitempty" yaml:"number,omitempty"`
}
