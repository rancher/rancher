package client

const (
	ResourceClaimType      = "resourceClaim"
	ResourceClaimFieldName = "name"
)

type ResourceClaim struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
