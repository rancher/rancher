package client

const (
	ServiceReferenceType           = "serviceReference"
	ServiceReferenceFieldName      = "name"
	ServiceReferenceFieldNamespace = "namespace"
	ServiceReferenceFieldPort      = "port"
)

type ServiceReference struct {
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Port      *int64 `json:"port,omitempty" yaml:"port,omitempty"`
}
