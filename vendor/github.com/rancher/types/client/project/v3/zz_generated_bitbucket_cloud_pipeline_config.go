package client

const (
	BitbucketCloudPipelineConfigType                 = "bitbucketCloudPipelineConfig"
	BitbucketCloudPipelineConfigFieldAnnotations     = "annotations"
	BitbucketCloudPipelineConfigFieldClientID        = "clientId"
	BitbucketCloudPipelineConfigFieldClientSecret    = "clientSecret"
	BitbucketCloudPipelineConfigFieldCreated         = "created"
	BitbucketCloudPipelineConfigFieldCreatorID       = "creatorId"
	BitbucketCloudPipelineConfigFieldEnabled         = "enabled"
	BitbucketCloudPipelineConfigFieldLabels          = "labels"
	BitbucketCloudPipelineConfigFieldName            = "name"
	BitbucketCloudPipelineConfigFieldNamespaceId     = "namespaceId"
	BitbucketCloudPipelineConfigFieldOwnerReferences = "ownerReferences"
	BitbucketCloudPipelineConfigFieldProjectID       = "projectId"
	BitbucketCloudPipelineConfigFieldRedirectURL     = "redirectUrl"
	BitbucketCloudPipelineConfigFieldRemoved         = "removed"
	BitbucketCloudPipelineConfigFieldType            = "type"
	BitbucketCloudPipelineConfigFieldUUID            = "uuid"
)

type BitbucketCloudPipelineConfig struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClientID        string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret    string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled         bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	RedirectURL     string            `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
