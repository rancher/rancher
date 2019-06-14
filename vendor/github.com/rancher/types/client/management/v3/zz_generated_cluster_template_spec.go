package client

const (
	ClusterTemplateSpecType                   = "clusterTemplateSpec"
	ClusterTemplateSpecFieldDefaultRevisionID = "defaultRevisionId"
	ClusterTemplateSpecFieldDescription       = "description"
	ClusterTemplateSpecFieldDisplayName       = "displayName"
	ClusterTemplateSpecFieldEnabled           = "enabled"
	ClusterTemplateSpecFieldMembers           = "members"
)

type ClusterTemplateSpec struct {
	DefaultRevisionID string   `json:"defaultRevisionId,omitempty" yaml:"defaultRevisionId,omitempty"`
	Description       string   `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName       string   `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Enabled           *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Members           []Member `json:"members,omitempty" yaml:"members,omitempty"`
}
