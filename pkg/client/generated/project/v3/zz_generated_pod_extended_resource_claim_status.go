package client

const (
	PodExtendedResourceClaimStatusType                   = "podExtendedResourceClaimStatus"
	PodExtendedResourceClaimStatusFieldRequestMappings   = "requestMappings"
	PodExtendedResourceClaimStatusFieldResourceClaimName = "resourceClaimName"
)

type PodExtendedResourceClaimStatus struct {
	RequestMappings   []ContainerExtendedResourceRequest `json:"requestMappings,omitempty" yaml:"requestMappings,omitempty"`
	ResourceClaimName string                             `json:"resourceClaimName,omitempty" yaml:"resourceClaimName,omitempty"`
}
