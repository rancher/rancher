package client

const (
	GenericOIDCConfigType                     = "genericOIDCConfig"
	GenericOIDCConfigFieldAccessMode          = "accessMode"
	GenericOIDCConfigFieldAcrValue            = "acrValue"
	GenericOIDCConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	GenericOIDCConfigFieldAnnotations         = "annotations"
	GenericOIDCConfigFieldAuthEndpoint        = "authEndpoint"
	GenericOIDCConfigFieldCertificate         = "certificate"
	GenericOIDCConfigFieldClientID            = "clientId"
	GenericOIDCConfigFieldClientSecret        = "clientSecret"
	GenericOIDCConfigFieldCreated             = "created"
	GenericOIDCConfigFieldCreatorID           = "creatorId"
	GenericOIDCConfigFieldEnabled             = "enabled"
	GenericOIDCConfigFieldGroupSearchEnabled  = "groupSearchEnabled"
	GenericOIDCConfigFieldGroupsClaim         = "groupsClaim"
	GenericOIDCConfigFieldIssuer              = "issuer"
	GenericOIDCConfigFieldJWKSUrl             = "jwksUrl"
	GenericOIDCConfigFieldLabels              = "labels"
	GenericOIDCConfigFieldName                = "name"
	GenericOIDCConfigFieldOwnerReferences     = "ownerReferences"
	GenericOIDCConfigFieldPrivateKey          = "privateKey"
	GenericOIDCConfigFieldRancherURL          = "rancherUrl"
	GenericOIDCConfigFieldRemoved             = "removed"
	GenericOIDCConfigFieldScopes              = "scope"
	GenericOIDCConfigFieldStatus              = "status"
	GenericOIDCConfigFieldTokenEndpoint       = "tokenEndpoint"
	GenericOIDCConfigFieldType                = "type"
	GenericOIDCConfigFieldUUID                = "uuid"
	GenericOIDCConfigFieldUserInfoEndpoint    = "userInfoEndpoint"
)

type GenericOIDCConfig struct {
	AccessMode          string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AcrValue            string            `json:"acrValue,omitempty" yaml:"acrValue,omitempty"`
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
	Scopes              string            `json:"scope,omitempty" yaml:"scope,omitempty"`
	Status              *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	TokenEndpoint       string            `json:"tokenEndpoint,omitempty" yaml:"tokenEndpoint,omitempty"`
	Type                string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserInfoEndpoint    string            `json:"userInfoEndpoint,omitempty" yaml:"userInfoEndpoint,omitempty"`
}
