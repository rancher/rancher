package client

const (
	TypedObjectReferenceType           = "typedObjectReference"
	TypedObjectReferenceFieldAPIGroup  = "apiGroup"
	TypedObjectReferenceFieldKind      = "kind"
	TypedObjectReferenceFieldName      = "name"
	TypedObjectReferenceFieldNamespace = "namespace"
)

type TypedObjectReference struct {
	APIGroup  string `json:"apiGroup,omitempty" yaml:"apiGroup,omitempty"`
	Kind      string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}
