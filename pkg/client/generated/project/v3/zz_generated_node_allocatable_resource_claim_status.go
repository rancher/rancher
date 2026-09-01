package client

const (
	NodeAllocatableResourceClaimStatusType                   = "nodeAllocatableResourceClaimStatus"
	NodeAllocatableResourceClaimStatusFieldContainers        = "containers"
	NodeAllocatableResourceClaimStatusFieldMapping           = "mapping"
	NodeAllocatableResourceClaimStatusFieldOverhead          = "overhead"
	NodeAllocatableResourceClaimStatusFieldResourceClaimName = "resourceClaimName"
)

type NodeAllocatableResourceClaimStatus struct {
	Containers        []string                           `json:"containers,omitempty" yaml:"containers,omitempty"`
	Mapping           []NodeAllocatableMappedResources   `json:"mapping,omitempty" yaml:"mapping,omitempty"`
	Overhead          []NodeAllocatableOverheadResources `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	ResourceClaimName string                             `json:"resourceClaimName,omitempty" yaml:"resourceClaimName,omitempty"`
}
