package client

const (
	ResourceStatusType           = "resourceStatus"
	ResourceStatusFieldName      = "name"
	ResourceStatusFieldResources = "resources"
)

type ResourceStatus struct {
	Name      string           `json:"name,omitempty" yaml:"name,omitempty"`
	Resources []ResourceHealth `json:"resources,omitempty" yaml:"resources,omitempty"`
}
