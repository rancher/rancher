package client

const (
	VolumeResourceRequirementsType          = "volumeResourceRequirements"
	VolumeResourceRequirementsFieldLimits   = "limits"
	VolumeResourceRequirementsFieldRequests = "requests"
)

type VolumeResourceRequirements struct {
	Limits   map[string]string `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty" yaml:"requests,omitempty"`
}
