package client

const (
	LocalObjectReferenceType      = "localObjectReference"
	LocalObjectReferenceFieldName = "name"
)

type LocalObjectReference struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
