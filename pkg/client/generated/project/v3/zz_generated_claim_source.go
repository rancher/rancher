package client

const (
	ClaimSourceType                           = "claimSource"
	ClaimSourceFieldResourceClaimName         = "resourceClaimName"
	ClaimSourceFieldResourceClaimTemplateName = "resourceClaimTemplateName"
)

type ClaimSource struct {
	ResourceClaimName         string `json:"resourceClaimName,omitempty" yaml:"resourceClaimName,omitempty"`
	ResourceClaimTemplateName string `json:"resourceClaimTemplateName,omitempty" yaml:"resourceClaimTemplateName,omitempty"`
}
