package client

const (
	ResourceRequirementsType          = "resourceRequirements"
	ResourceRequirementsFieldClaims   = "claims"
	ResourceRequirementsFieldLimits   = "limits"
	ResourceRequirementsFieldRequests = "requests"
)

type ResourceRequirements struct {
	Claims   []ResourceClaim   `json:"claims,omitempty" yaml:"claims,omitempty"`
	Limits   map[string]string `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty" yaml:"requests,omitempty"`
}
