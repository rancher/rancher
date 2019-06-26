package client

const (
	ServiceReferenceType           = "serviceReference"
	ServiceReferenceFieldName      = "name"
	ServiceReferenceFieldNamespace = "namespace"
)

type ServiceReference struct {
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}
