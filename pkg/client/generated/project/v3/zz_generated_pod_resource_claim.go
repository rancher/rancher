package client

const (
	PodResourceClaimType                           = "podResourceClaim"
	PodResourceClaimFieldName                      = "name"
	PodResourceClaimFieldResourceClaimName         = "resourceClaimName"
	PodResourceClaimFieldResourceClaimTemplateName = "resourceClaimTemplateName"
)

type PodResourceClaim struct {
	Name                      string `json:"name,omitempty" yaml:"name,omitempty"`
	ResourceClaimName         string `json:"resourceClaimName,omitempty" yaml:"resourceClaimName,omitempty"`
	ResourceClaimTemplateName string `json:"resourceClaimTemplateName,omitempty" yaml:"resourceClaimTemplateName,omitempty"`
}
