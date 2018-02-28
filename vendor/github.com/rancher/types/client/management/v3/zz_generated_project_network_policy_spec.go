package client

const (
	ProjectNetworkPolicySpecType             = "projectNetworkPolicySpec"
	ProjectNetworkPolicySpecFieldDescription = "description"
	ProjectNetworkPolicySpecFieldProjectId   = "projectId"
)

type ProjectNetworkPolicySpec struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	ProjectId   string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
}
