package client

const (
	NodeAllocatableResourceClaimStatusType                   = "nodeAllocatableResourceClaimStatus"
	NodeAllocatableResourceClaimStatusFieldContainers        = "containers"
	NodeAllocatableResourceClaimStatusFieldResourceClaimName = "resourceClaimName"
	NodeAllocatableResourceClaimStatusFieldResources         = "resources"
)

type NodeAllocatableResourceClaimStatus struct {
	Containers        []string          `json:"containers,omitempty" yaml:"containers,omitempty"`
	ResourceClaimName string            `json:"resourceClaimName,omitempty" yaml:"resourceClaimName,omitempty"`
	Resources         map[string]string `json:"resources,omitempty" yaml:"resources,omitempty"`
}
