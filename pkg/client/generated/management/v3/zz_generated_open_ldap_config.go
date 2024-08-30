package client

const (
	OpenLdapConfigType                                 = "openLdapConfig"
	OpenLdapConfigFieldAccessMode                      = "accessMode"
	OpenLdapConfigFieldAllowedPrincipalIDs             = "allowedPrincipalIds"
	OpenLdapConfigFieldAnnotations                     = "annotations"
	OpenLdapConfigFieldCertificate                     = "certificate"
	OpenLdapConfigFieldConnectionTimeout               = "connectionTimeout"
	OpenLdapConfigFieldCreated                         = "created"
	OpenLdapConfigFieldCreatorID                       = "creatorId"
	OpenLdapConfigFieldEnabled                         = "enabled"
	OpenLdapConfigFieldGroupDNAttribute                = "groupDNAttribute"
	OpenLdapConfigFieldGroupMemberMappingAttribute     = "groupMemberMappingAttribute"
	OpenLdapConfigFieldGroupMemberUserAttribute        = "groupMemberUserAttribute"
	OpenLdapConfigFieldGroupNameAttribute              = "groupNameAttribute"
	OpenLdapConfigFieldGroupObjectClass                = "groupObjectClass"
	OpenLdapConfigFieldGroupSearchAttribute            = "groupSearchAttribute"
	OpenLdapConfigFieldGroupSearchBase                 = "groupSearchBase"
	OpenLdapConfigFieldGroupSearchFilter               = "groupSearchFilter"
	OpenLdapConfigFieldLabels                          = "labels"
	OpenLdapConfigFieldLogoutAllSupported              = "logoutAllSupported"
	OpenLdapConfigFieldName                            = "name"
	OpenLdapConfigFieldNestedGroupMembershipEnabled    = "nestedGroupMembershipEnabled"
	OpenLdapConfigFieldOwnerReferences                 = "ownerReferences"
	OpenLdapConfigFieldPort                            = "port"
	OpenLdapConfigFieldRemoved                         = "removed"
	OpenLdapConfigFieldSearchUsingServiceAccount       = "searchUsingServiceAccount"
	OpenLdapConfigFieldServers                         = "servers"
	OpenLdapConfigFieldServiceAccountDistinguishedName = "serviceAccountDistinguishedName"
	OpenLdapConfigFieldServiceAccountPassword          = "serviceAccountPassword"
	OpenLdapConfigFieldStartTLS                        = "starttls"
	OpenLdapConfigFieldStatus                          = "status"
	OpenLdapConfigFieldTLS                             = "tls"
	OpenLdapConfigFieldType                            = "type"
	OpenLdapConfigFieldUUID                            = "uuid"
	OpenLdapConfigFieldUserDisabledBitMask             = "userDisabledBitMask"
	OpenLdapConfigFieldUserEnabledAttribute            = "userEnabledAttribute"
	OpenLdapConfigFieldUserLoginAttribute              = "userLoginAttribute"
	OpenLdapConfigFieldUserMemberAttribute             = "userMemberAttribute"
	OpenLdapConfigFieldUserNameAttribute               = "userNameAttribute"
	OpenLdapConfigFieldUserObjectClass                 = "userObjectClass"
	OpenLdapConfigFieldUserSearchAttribute             = "userSearchAttribute"
	OpenLdapConfigFieldUserSearchBase                  = "userSearchBase"
	OpenLdapConfigFieldUserSearchFilter                = "userSearchFilter"
)

type OpenLdapConfig struct {
	AccessMode                      string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowedPrincipalIDs             []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations                     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Certificate                     string            `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ConnectionTimeout               int64             `json:"connectionTimeout,omitempty" yaml:"connectionTimeout,omitempty"`
	Created                         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled                         bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GroupDNAttribute                string            `json:"groupDNAttribute,omitempty" yaml:"groupDNAttribute,omitempty"`
	GroupMemberMappingAttribute     string            `json:"groupMemberMappingAttribute,omitempty" yaml:"groupMemberMappingAttribute,omitempty"`
	GroupMemberUserAttribute        string            `json:"groupMemberUserAttribute,omitempty" yaml:"groupMemberUserAttribute,omitempty"`
	GroupNameAttribute              string            `json:"groupNameAttribute,omitempty" yaml:"groupNameAttribute,omitempty"`
	GroupObjectClass                string            `json:"groupObjectClass,omitempty" yaml:"groupObjectClass,omitempty"`
	GroupSearchAttribute            string            `json:"groupSearchAttribute,omitempty" yaml:"groupSearchAttribute,omitempty"`
	GroupSearchBase                 string            `json:"groupSearchBase,omitempty" yaml:"groupSearchBase,omitempty"`
	GroupSearchFilter               string            `json:"groupSearchFilter,omitempty" yaml:"groupSearchFilter,omitempty"`
	Labels                          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LogoutAllSupported              bool              `json:"logoutAllSupported,omitempty" yaml:"logoutAllSupported,omitempty"`
	Name                            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NestedGroupMembershipEnabled    bool              `json:"nestedGroupMembershipEnabled,omitempty" yaml:"nestedGroupMembershipEnabled,omitempty"`
	OwnerReferences                 []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Port                            int64             `json:"port,omitempty" yaml:"port,omitempty"`
	Removed                         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SearchUsingServiceAccount       bool              `json:"searchUsingServiceAccount,omitempty" yaml:"searchUsingServiceAccount,omitempty"`
	Servers                         []string          `json:"servers,omitempty" yaml:"servers,omitempty"`
	ServiceAccountDistinguishedName string            `json:"serviceAccountDistinguishedName,omitempty" yaml:"serviceAccountDistinguishedName,omitempty"`
	ServiceAccountPassword          string            `json:"serviceAccountPassword,omitempty" yaml:"serviceAccountPassword,omitempty"`
	StartTLS                        bool              `json:"starttls,omitempty" yaml:"starttls,omitempty"`
	Status                          *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	TLS                             bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	Type                            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserDisabledBitMask             int64             `json:"userDisabledBitMask,omitempty" yaml:"userDisabledBitMask,omitempty"`
	UserEnabledAttribute            string            `json:"userEnabledAttribute,omitempty" yaml:"userEnabledAttribute,omitempty"`
	UserLoginAttribute              string            `json:"userLoginAttribute,omitempty" yaml:"userLoginAttribute,omitempty"`
	UserMemberAttribute             string            `json:"userMemberAttribute,omitempty" yaml:"userMemberAttribute,omitempty"`
	UserNameAttribute               string            `json:"userNameAttribute,omitempty" yaml:"userNameAttribute,omitempty"`
	UserObjectClass                 string            `json:"userObjectClass,omitempty" yaml:"userObjectClass,omitempty"`
	UserSearchAttribute             string            `json:"userSearchAttribute,omitempty" yaml:"userSearchAttribute,omitempty"`
	UserSearchBase                  string            `json:"userSearchBase,omitempty" yaml:"userSearchBase,omitempty"`
	UserSearchFilter                string            `json:"userSearchFilter,omitempty" yaml:"userSearchFilter,omitempty"`
}
