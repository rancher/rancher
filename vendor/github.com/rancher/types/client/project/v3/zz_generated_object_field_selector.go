package client

const (
	ObjectFieldSelectorType            = "objectFieldSelector"
	ObjectFieldSelectorFieldAPIVersion = "apiVersion"
	ObjectFieldSelectorFieldFieldPath  = "fieldPath"
)

type ObjectFieldSelector struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	FieldPath  string `json:"fieldPath,omitempty" yaml:"fieldPath,omitempty"`
}
