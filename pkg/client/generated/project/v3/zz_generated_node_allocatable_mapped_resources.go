package client

const (
	NodeAllocatableMappedResourcesType          = "nodeAllocatableMappedResources"
	NodeAllocatableMappedResourcesFieldName     = "name"
	NodeAllocatableMappedResourcesFieldQuantity = "quantity"
)

type NodeAllocatableMappedResources struct {
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Quantity string `json:"quantity,omitempty" yaml:"quantity,omitempty"`
}
