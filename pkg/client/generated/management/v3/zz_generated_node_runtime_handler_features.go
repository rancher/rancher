package client

const (
	NodeRuntimeHandlerFeaturesType                         = "nodeRuntimeHandlerFeatures"
	NodeRuntimeHandlerFeaturesFieldRecursiveReadOnlyMounts = "recursiveReadOnlyMounts"
)

type NodeRuntimeHandlerFeatures struct {
	RecursiveReadOnlyMounts *bool `json:"recursiveReadOnlyMounts,omitempty" yaml:"recursiveReadOnlyMounts,omitempty"`
}
