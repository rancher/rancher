package client

const (
	GitlabPipelineConfigType                 = "gitlabPipelineConfig"
	GitlabPipelineConfigFieldAnnotations     = "annotations"
	GitlabPipelineConfigFieldClientID        = "clientId"
	GitlabPipelineConfigFieldClientSecret    = "clientSecret"
	GitlabPipelineConfigFieldCreated         = "created"
	GitlabPipelineConfigFieldCreatorID       = "creatorId"
	GitlabPipelineConfigFieldEnabled         = "enabled"
	GitlabPipelineConfigFieldHostname        = "hostname"
	GitlabPipelineConfigFieldLabels          = "labels"
	GitlabPipelineConfigFieldName            = "name"
	GitlabPipelineConfigFieldNamespaceId     = "namespaceId"
	GitlabPipelineConfigFieldOwnerReferences = "ownerReferences"
	GitlabPipelineConfigFieldProjectID       = "projectId"
	GitlabPipelineConfigFieldRedirectURL     = "redirectUrl"
	GitlabPipelineConfigFieldRemoved         = "removed"
	GitlabPipelineConfigFieldTLS             = "tls"
	GitlabPipelineConfigFieldType            = "type"
	GitlabPipelineConfigFieldUUID            = "uuid"
)

type GitlabPipelineConfig struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClientID        string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret    string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled         bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Hostname        string            `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	RedirectURL     string            `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	TLS             bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
