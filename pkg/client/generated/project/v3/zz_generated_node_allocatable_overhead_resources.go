package client

const (
	NodeAllocatableOverheadResourcesType              = "nodeAllocatableOverheadResources"
	NodeAllocatableOverheadResourcesFieldName         = "name"
	NodeAllocatableOverheadResourcesFieldPerContainer = "perContainer"
	NodeAllocatableOverheadResourcesFieldPerPod       = "perPod"
)

type NodeAllocatableOverheadResources struct {
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	PerContainer string `json:"perContainer,omitempty" yaml:"perContainer,omitempty"`
	PerPod       string `json:"perPod,omitempty" yaml:"perPod,omitempty"`
}
