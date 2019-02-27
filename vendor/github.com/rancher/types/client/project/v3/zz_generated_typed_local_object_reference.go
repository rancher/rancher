package client

const (
	TypedLocalObjectReferenceType          = "typedLocalObjectReference"
	TypedLocalObjectReferenceFieldAPIGroup = "apiGroup"
	TypedLocalObjectReferenceFieldKind     = "kind"
	TypedLocalObjectReferenceFieldName     = "name"
)

type TypedLocalObjectReference struct {
	APIGroup string `json:"apiGroup,omitempty" yaml:"apiGroup,omitempty"`
	Kind     string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
}
