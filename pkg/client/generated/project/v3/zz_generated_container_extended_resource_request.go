package client

const (
	ContainerExtendedResourceRequestType               = "containerExtendedResourceRequest"
	ContainerExtendedResourceRequestFieldContainerName = "containerName"
	ContainerExtendedResourceRequestFieldRequestName   = "requestName"
	ContainerExtendedResourceRequestFieldResourceName  = "resourceName"
)

type ContainerExtendedResourceRequest struct {
	ContainerName string `json:"containerName,omitempty" yaml:"containerName,omitempty"`
	RequestName   string `json:"requestName,omitempty" yaml:"requestName,omitempty"`
	ResourceName  string `json:"resourceName,omitempty" yaml:"resourceName,omitempty"`
}
