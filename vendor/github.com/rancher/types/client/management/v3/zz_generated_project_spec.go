package client

const (
	ProjectSpecType             = "projectSpec"
	ProjectSpecFieldClusterId   = "clusterId"
	ProjectSpecFieldDisplayName = "displayName"
)

type ProjectSpec struct {
	ClusterId   string `json:"clusterId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}
