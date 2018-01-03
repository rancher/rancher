package client

const (
	ProjectSpecType                             = "projectSpec"
	ProjectSpecFieldClusterId                   = "clusterId"
	ProjectSpecFieldDescription                 = "description"
	ProjectSpecFieldDisplayName                 = "displayName"
	ProjectSpecFieldPodSecurityPolicyTemplateId = "podSecurityPolicyTemplateId"
)

type ProjectSpec struct {
	ClusterId                   string `json:"clusterId,omitempty"`
	Description                 string `json:"description,omitempty"`
	DisplayName                 string `json:"displayName,omitempty"`
	PodSecurityPolicyTemplateId string `json:"podSecurityPolicyTemplateId,omitempty"`
}
