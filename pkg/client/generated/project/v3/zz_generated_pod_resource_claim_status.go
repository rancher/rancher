package client

const (
	PodResourceClaimStatusType                   = "podResourceClaimStatus"
	PodResourceClaimStatusFieldName              = "name"
	PodResourceClaimStatusFieldResourceClaimName = "resourceClaimName"
)

type PodResourceClaimStatus struct {
	Name              string `json:"name,omitempty" yaml:"name,omitempty"`
	ResourceClaimName string `json:"resourceClaimName,omitempty" yaml:"resourceClaimName,omitempty"`
}
