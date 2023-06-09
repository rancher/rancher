package client

const (
	PodResourceClaimType        = "podResourceClaim"
	PodResourceClaimFieldName   = "name"
	PodResourceClaimFieldSource = "source"
)

type PodResourceClaim struct {
	Name   string       `json:"name,omitempty" yaml:"name,omitempty"`
	Source *ClaimSource `json:"source,omitempty" yaml:"source,omitempty"`
}
