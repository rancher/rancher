package client

const (
	ProjectNetworkPolicySpecType             = "projectNetworkPolicySpec"
	ProjectNetworkPolicySpecFieldDescription = "description"
	ProjectNetworkPolicySpecFieldProjectID   = "projectId"
)

type ProjectNetworkPolicySpec struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	ProjectID   string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
}
