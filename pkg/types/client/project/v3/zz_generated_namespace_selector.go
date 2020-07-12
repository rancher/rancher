package client

const (
	NamespaceSelectorType            = "namespaceSelector"
	NamespaceSelectorFieldAny        = "any"
	NamespaceSelectorFieldMatchNames = "matchNames"
)

type NamespaceSelector struct {
	Any        bool     `json:"any,omitempty" yaml:"any,omitempty"`
	MatchNames []string `json:"matchNames,omitempty" yaml:"matchNames,omitempty"`
}
