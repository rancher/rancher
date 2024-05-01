package client

const (
	KeyCloakOIDCConfigType                     = "keyCloakOIDCConfig"
	KeyCloakOIDCConfigFieldAccessMode          = "accessMode"
	KeyCloakOIDCConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	KeyCloakOIDCConfigFieldAnnotations         = "annotations"
	KeyCloakOIDCConfigFieldAuthEndpoint        = "authEndpoint"
	KeyCloakOIDCConfigFieldCertificate         = "certificate"
	KeyCloakOIDCConfigFieldClientID            = "clientId"
	KeyCloakOIDCConfigFieldClientSecret        = "clientSecret"
	KeyCloakOIDCConfigFieldCreated             = "created"
	KeyCloakOIDCConfigFieldCreatorID           = "creatorId"
	KeyCloakOIDCConfigFieldEnabled             = "enabled"
	KeyCloakOIDCConfigFieldGroupSearchEnabled  = "groupSearchEnabled"
	KeyCloakOIDCConfigFieldGroupsClaim         = "groupsClaim"
	KeyCloakOIDCConfigFieldIssuer              = "issuer"
	KeyCloakOIDCConfigFieldJWKSUrl             = "jwksUrl"
	KeyCloakOIDCConfigFieldLabels              = "labels"
	KeyCloakOIDCConfigFieldName                = "name"
	KeyCloakOIDCConfigFieldOwnerReferences     = "ownerReferences"
	KeyCloakOIDCConfigFieldPrivateKey          = "privateKey"
	KeyCloakOIDCConfigFieldRancherURL          = "rancherUrl"
	KeyCloakOIDCConfigFieldRemoved             = "removed"
	KeyCloakOIDCConfigFieldScopes              = "scopes"
	KeyCloakOIDCConfigFieldStatus              = "status"
	KeyCloakOIDCConfigFieldType                = "type"
	KeyCloakOIDCConfigFieldUUID                = "uuid"
)

type KeyCloakOIDCConfig struct {
	AccessMode          string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowedPrincipalIDs []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthEndpoint        string            `json:"authEndpoint,omitempty" yaml:"authEndpoint,omitempty"`
	Certificate         string            `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ClientID            string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret        string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Created             string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled             bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GroupSearchEnabled  *bool             `json:"groupSearchEnabled,omitempty" yaml:"groupSearchEnabled,omitempty"`
	GroupsClaim         string            `json:"groupsClaim,omitempty" yaml:"groupsClaim,omitempty"`
	Issuer              string            `json:"issuer,omitempty" yaml:"issuer,omitempty"`
	JWKSUrl             string            `json:"jwksUrl,omitempty" yaml:"jwksUrl,omitempty"`
	Labels              map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PrivateKey          string            `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`
	RancherURL          string            `json:"rancherUrl,omitempty" yaml:"rancherUrl,omitempty"`
	Removed             string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Scopes              string            `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Status              *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Type                string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
