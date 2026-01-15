package client

const (
	ContractVersionedObjectReferenceType          = "contractVersionedObjectReference"
	ContractVersionedObjectReferenceFieldAPIGroup = "apiGroup"
	ContractVersionedObjectReferenceFieldKind     = "kind"
	ContractVersionedObjectReferenceFieldName     = "name"
)

type ContractVersionedObjectReference struct {
	APIGroup string `json:"apiGroup,omitempty" yaml:"apiGroup,omitempty"`
	Kind     string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
}
