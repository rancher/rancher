package client

const (
	CrossVersionObjectReferenceType            = "crossVersionObjectReference"
	CrossVersionObjectReferenceFieldAPIVersion = "apiVersion"
	CrossVersionObjectReferenceFieldKind       = "kind"
	CrossVersionObjectReferenceFieldName       = "name"
)

type CrossVersionObjectReference struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
}
