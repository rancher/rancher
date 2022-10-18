package client

const (
	ContainerResourceLimitType                = "containerResourceLimit"
	ContainerResourceLimitFieldLimitsCPU      = "limitsCpu"
	ContainerResourceLimitFieldLimitsMemory   = "limitsMemory"
	ContainerResourceLimitFieldRequestsCPU    = "requestsCpu"
	ContainerResourceLimitFieldRequestsMemory = "requestsMemory"
)

type ContainerResourceLimit struct {
	LimitsCPU      string `json:"limitsCpu,omitempty" yaml:"limitsCpu,omitempty"`
	LimitsMemory   string `json:"limitsMemory,omitempty" yaml:"limitsMemory,omitempty"`
	RequestsCPU    string `json:"requestsCpu,omitempty" yaml:"requestsCpu,omitempty"`
	RequestsMemory string `json:"requestsMemory,omitempty" yaml:"requestsMemory,omitempty"`
}
