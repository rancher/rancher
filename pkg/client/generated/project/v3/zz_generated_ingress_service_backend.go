package client

const (
	IngressServiceBackendType      = "ingressServiceBackend"
	IngressServiceBackendFieldName = "name"
	IngressServiceBackendFieldPort = "port"
)

type IngressServiceBackend struct {
	Name string              `json:"name,omitempty" yaml:"name,omitempty"`
	Port *ServiceBackendPort `json:"port,omitempty" yaml:"port,omitempty"`
}
