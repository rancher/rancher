package client

const (
	ResourceRequirementsType          = "resourceRequirements"
	ResourceRequirementsFieldLimits   = "limits"
	ResourceRequirementsFieldRequests = "requests"
)

type ResourceRequirements struct {
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}
