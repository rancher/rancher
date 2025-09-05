package client

const (
	GithubAppConfigType                     = "githubAppConfig"
	GithubAppConfigFieldAccessMode          = "accessMode"
	GithubAppConfigFieldAdditionalClientIDs = "additionalClientIds"
	GithubAppConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	GithubAppConfigFieldAnnotations         = "annotations"
	GithubAppConfigFieldAppID               = "appId"
	GithubAppConfigFieldClientID            = "clientId"
	GithubAppConfigFieldClientSecret        = "clientSecret"
	GithubAppConfigFieldCreated             = "created"
	GithubAppConfigFieldCreatorID           = "creatorId"
	GithubAppConfigFieldEnabled             = "enabled"
	GithubAppConfigFieldHostname            = "hostname"
	GithubAppConfigFieldHostnameToClientID  = "hostnameToClientId"
	GithubAppConfigFieldInstallationID      = "installationId"
	GithubAppConfigFieldLabels              = "labels"
	GithubAppConfigFieldLogoutAllSupported  = "logoutAllSupported"
	GithubAppConfigFieldName                = "name"
	GithubAppConfigFieldOwnerReferences     = "ownerReferences"
	GithubAppConfigFieldPrivateKey          = "privateKey"
	GithubAppConfigFieldRemoved             = "removed"
	GithubAppConfigFieldStatus              = "status"
	GithubAppConfigFieldTLS                 = "tls"
	GithubAppConfigFieldType                = "type"
	GithubAppConfigFieldUUID                = "uuid"
)

type GithubAppConfig struct {
	AccessMode          string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AdditionalClientIDs map[string]string `json:"additionalClientIds,omitempty" yaml:"additionalClientIds,omitempty"`
	AllowedPrincipalIDs []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AppID               string            `json:"appId,omitempty" yaml:"appId,omitempty"`
	ClientID            string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret        string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Created             string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled             bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Hostname            string            `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	HostnameToClientID  map[string]string `json:"hostnameToClientId,omitempty" yaml:"hostnameToClientId,omitempty"`
	InstallationID      string            `json:"installationId,omitempty" yaml:"installationId,omitempty"`
	Labels              map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LogoutAllSupported  bool              `json:"logoutAllSupported,omitempty" yaml:"logoutAllSupported,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PrivateKey          string            `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`
	Removed             string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Status              *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	TLS                 bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	Type                string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
