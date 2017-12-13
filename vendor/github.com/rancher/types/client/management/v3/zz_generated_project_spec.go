package client

const (
	ProjectSpecType                             = "projectSpec"
	ProjectSpecFieldClusterId                   = "clusterId"
	ProjectSpecFieldDisplayName                 = "displayName"
	ProjectSpecFieldPodSecurityPolicyTemplateId = "podSecurityPolicyTemplateId"
)

type ProjectSpec struct {
	ClusterId                   string `json:"clusterId,omitempty"`
	DisplayName                 string `json:"displayName,omitempty"`
	PodSecurityPolicyTemplateId string `json:"podSecurityPolicyTemplateId,omitempty"`
}
