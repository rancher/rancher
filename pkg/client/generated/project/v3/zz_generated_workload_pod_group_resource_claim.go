package client

const (
	WorkloadPodGroupResourceClaimType                           = "workloadPodGroupResourceClaim"
	WorkloadPodGroupResourceClaimFieldName                      = "name"
	WorkloadPodGroupResourceClaimFieldResourceClaimName         = "resourceClaimName"
	WorkloadPodGroupResourceClaimFieldResourceClaimTemplateName = "resourceClaimTemplateName"
)

type WorkloadPodGroupResourceClaim struct {
	Name                      string `json:"name,omitempty" yaml:"name,omitempty"`
	ResourceClaimName         string `json:"resourceClaimName,omitempty" yaml:"resourceClaimName,omitempty"`
	ResourceClaimTemplateName string `json:"resourceClaimTemplateName,omitempty" yaml:"resourceClaimTemplateName,omitempty"`
}
