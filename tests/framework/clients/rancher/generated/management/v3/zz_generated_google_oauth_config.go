package client

const (
	GoogleOauthConfigType                              = "googleOauthConfig"
	GoogleOauthConfigFieldAccessMode                   = "accessMode"
	GoogleOauthConfigFieldAdminEmail                   = "adminEmail"
	GoogleOauthConfigFieldAllowedPrincipalIDs          = "allowedPrincipalIds"
	GoogleOauthConfigFieldAnnotations                  = "annotations"
	GoogleOauthConfigFieldCreated                      = "created"
	GoogleOauthConfigFieldCreatorID                    = "creatorId"
	GoogleOauthConfigFieldEnabled                      = "enabled"
	GoogleOauthConfigFieldHostname                     = "hostname"
	GoogleOauthConfigFieldLabels                       = "labels"
	GoogleOauthConfigFieldName                         = "name"
	GoogleOauthConfigFieldNestedGroupMembershipEnabled = "nestedGroupMembershipEnabled"
	GoogleOauthConfigFieldOauthCredential              = "oauthCredential"
	GoogleOauthConfigFieldOwnerReferences              = "ownerReferences"
	GoogleOauthConfigFieldRemoved                      = "removed"
	GoogleOauthConfigFieldServiceAccountCredential     = "serviceAccountCredential"
	GoogleOauthConfigFieldStatus                       = "status"
	GoogleOauthConfigFieldType                         = "type"
	GoogleOauthConfigFieldUUID                         = "uuid"
	GoogleOauthConfigFieldUserInfoEndpoint             = "userInfoEndpoint"
)

type GoogleOauthConfig struct {
	AccessMode                   string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AdminEmail                   string            `json:"adminEmail,omitempty" yaml:"adminEmail,omitempty"`
	AllowedPrincipalIDs          []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations                  map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created                      string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                    string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled                      bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Hostname                     string            `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Labels                       map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                         string            `json:"name,omitempty" yaml:"name,omitempty"`
	NestedGroupMembershipEnabled bool              `json:"nestedGroupMembershipEnabled,omitempty" yaml:"nestedGroupMembershipEnabled,omitempty"`
	OauthCredential              string            `json:"oauthCredential,omitempty" yaml:"oauthCredential,omitempty"`
	OwnerReferences              []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed                      string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	ServiceAccountCredential     string            `json:"serviceAccountCredential,omitempty" yaml:"serviceAccountCredential,omitempty"`
	Status                       *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Type                         string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                         string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserInfoEndpoint             string            `json:"userInfoEndpoint,omitempty" yaml:"userInfoEndpoint,omitempty"`
}
