package client

const (
	NodeRuntimeHandlerType          = "nodeRuntimeHandler"
	NodeRuntimeHandlerFieldFeatures = "features"
	NodeRuntimeHandlerFieldName     = "name"
)

type NodeRuntimeHandler struct {
	Features *NodeRuntimeHandlerFeatures `json:"features,omitempty" yaml:"features,omitempty"`
	Name     string                      `json:"name,omitempty" yaml:"name,omitempty"`
}
