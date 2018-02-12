package client

const (
	GithubConfigType                     = "githubConfig"
	GithubConfigFieldAccessMode          = "accessMode"
	GithubConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	GithubConfigFieldAnnotations         = "annotations"
	GithubConfigFieldClientID            = "clientId"
	GithubConfigFieldClientSecret        = "clientSecret"
	GithubConfigFieldCreated             = "created"
	GithubConfigFieldCreatorID           = "creatorId"
	GithubConfigFieldEnabled             = "enabled"
	GithubConfigFieldHostname            = "hostname"
	GithubConfigFieldLabels              = "labels"
	GithubConfigFieldName                = "name"
	GithubConfigFieldOwnerReferences     = "ownerReferences"
	GithubConfigFieldRemoved             = "removed"
	GithubConfigFieldTLS                 = "tls"
	GithubConfigFieldType                = "type"
	GithubConfigFieldUuid                = "uuid"
)

type GithubConfig struct {
	AccessMode          string            `json:"accessMode,omitempty"`
	AllowedPrincipalIDs []string          `json:"allowedPrincipalIds,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty"`
	ClientID            string            `json:"clientId,omitempty"`
	ClientSecret        string            `json:"clientSecret,omitempty"`
	Created             string            `json:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty"`
	Enabled             *bool             `json:"enabled,omitempty"`
	Hostname            string            `json:"hostname,omitempty"`
	Labels              map[string]string `json:"labels,omitempty"`
	Name                string            `json:"name,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed             string            `json:"removed,omitempty"`
	TLS                 bool              `json:"tls,omitempty"`
	Type                string            `json:"type,omitempty"`
	Uuid                string            `json:"uuid,omitempty"`
}
