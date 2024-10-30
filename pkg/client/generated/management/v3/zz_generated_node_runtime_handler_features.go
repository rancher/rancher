package client

const (
	NodeRuntimeHandlerFeaturesType                         = "nodeRuntimeHandlerFeatures"
	NodeRuntimeHandlerFeaturesFieldRecursiveReadOnlyMounts = "recursiveReadOnlyMounts"
	NodeRuntimeHandlerFeaturesFieldUserNamespaces          = "userNamespaces"
)

type NodeRuntimeHandlerFeatures struct {
	RecursiveReadOnlyMounts *bool `json:"recursiveReadOnlyMounts,omitempty" yaml:"recursiveReadOnlyMounts,omitempty"`
	UserNamespaces          *bool `json:"userNamespaces,omitempty" yaml:"userNamespaces,omitempty"`
}
