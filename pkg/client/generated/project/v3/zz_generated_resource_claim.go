package client

const (
	ResourceClaimType         = "resourceClaim"
	ResourceClaimFieldName    = "name"
	ResourceClaimFieldRequest = "request"
)

type ResourceClaim struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Request string `json:"request,omitempty" yaml:"request,omitempty"`
}
