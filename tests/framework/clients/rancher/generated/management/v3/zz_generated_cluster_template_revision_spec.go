package client

const (
	ClusterTemplateRevisionSpecType                   = "clusterTemplateRevisionSpec"
	ClusterTemplateRevisionSpecFieldClusterConfig     = "clusterConfig"
	ClusterTemplateRevisionSpecFieldClusterTemplateID = "clusterTemplateId"
	ClusterTemplateRevisionSpecFieldDisplayName       = "displayName"
	ClusterTemplateRevisionSpecFieldEnabled           = "enabled"
	ClusterTemplateRevisionSpecFieldQuestions         = "questions"
)

type ClusterTemplateRevisionSpec struct {
	ClusterConfig     *ClusterSpecBase `json:"clusterConfig,omitempty" yaml:"clusterConfig,omitempty"`
	ClusterTemplateID string           `json:"clusterTemplateId,omitempty" yaml:"clusterTemplateId,omitempty"`
	DisplayName       string           `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Enabled           *bool            `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Questions         []Question       `json:"questions,omitempty" yaml:"questions,omitempty"`
}
