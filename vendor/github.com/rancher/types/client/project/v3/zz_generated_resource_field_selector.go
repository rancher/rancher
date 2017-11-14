package client

const (
	ResourceFieldSelectorType               = "resourceFieldSelector"
	ResourceFieldSelectorFieldContainerName = "containerName"
	ResourceFieldSelectorFieldDivisor       = "divisor"
	ResourceFieldSelectorFieldResource      = "resource"
)

type ResourceFieldSelector struct {
	ContainerName string `json:"containerName,omitempty"`
	Divisor       string `json:"divisor,omitempty"`
	Resource      string `json:"resource,omitempty"`
}
