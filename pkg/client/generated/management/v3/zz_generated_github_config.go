package client

const (
	GithubConfigType                     = "githubConfig"
	GithubConfigFieldAccessMode          = "accessMode"
	GithubConfigFieldAdditionalClientIDs = "additionalClientIds"
	GithubConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	GithubConfigFieldAnnotations         = "annotations"
	GithubConfigFieldClientID            = "clientId"
	GithubConfigFieldClientSecret        = "clientSecret"
	GithubConfigFieldCreated             = "created"
	GithubConfigFieldCreatorID           = "creatorId"
	GithubConfigFieldEnabled             = "enabled"
	GithubConfigFieldHostname            = "hostname"
	GithubConfigFieldHostnameToClientID  = "hostnameToClientId"
	GithubConfigFieldLabels              = "labels"
	GithubConfigFieldLogoutAllSupported  = "logoutAllSupported"
	GithubConfigFieldName                = "name"
	GithubConfigFieldOwnerReferences     = "ownerReferences"
	GithubConfigFieldRemoved             = "removed"
	GithubConfigFieldStatus              = "status"
	GithubConfigFieldTLS                 = "tls"
	GithubConfigFieldType                = "type"
	GithubConfigFieldUUID                = "uuid"
)

type GithubConfig struct {
	AccessMode          string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AdditionalClientIDs map[string]string `json:"additionalClientIds,omitempty" yaml:"additionalClientIds,omitempty"`
	AllowedPrincipalIDs []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClientID            string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret        string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Created             string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled             bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Hostname            string            `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	HostnameToClientID  map[string]string `json:"hostnameToClientId,omitempty" yaml:"hostnameToClientId,omitempty"`
	Labels              map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LogoutAllSupported  bool              `json:"logoutAllSupported,omitempty" yaml:"logoutAllSupported,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed             string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Status              *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	TLS                 bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	Type                string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
