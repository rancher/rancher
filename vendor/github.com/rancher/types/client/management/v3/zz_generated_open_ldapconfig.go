package client

const (
	OpenLDAPConfigType                             = "openLDAPConfig"
	OpenLDAPConfigFieldAccessMode                  = "accessMode"
	OpenLDAPConfigFieldAllowedPrincipalIDs         = "allowedPrincipalIds"
	OpenLDAPConfigFieldAnnotations                 = "annotations"
	OpenLDAPConfigFieldCertificate                 = "certificate"
	OpenLDAPConfigFieldConnectionTimeout           = "connectionTimeout"
	OpenLDAPConfigFieldCreated                     = "created"
	OpenLDAPConfigFieldCreatorID                   = "creatorId"
	OpenLDAPConfigFieldEnabled                     = "enabled"
	OpenLDAPConfigFieldGroupDNAttribute            = "groupDNAttribute"
	OpenLDAPConfigFieldGroupMemberMappingAttribute = "groupMemberMappingAttribute"
	OpenLDAPConfigFieldGroupMemberUserAttribute    = "groupMemberUserAttribute"
	OpenLDAPConfigFieldGroupNameAttribute          = "groupNameAttribute"
	OpenLDAPConfigFieldGroupObjectClass            = "groupObjectClass"
	OpenLDAPConfigFieldGroupSearchAttribute        = "groupSearchAttribute"
	OpenLDAPConfigFieldGroupSearchBase             = "groupSearchBase"
	OpenLDAPConfigFieldLabels                      = "labels"
	OpenLDAPConfigFieldName                        = "name"
	OpenLDAPConfigFieldOwnerReferences             = "ownerReferences"
	OpenLDAPConfigFieldPort                        = "port"
	OpenLDAPConfigFieldRemoved                     = "removed"
	OpenLDAPConfigFieldServers                     = "servers"
	OpenLDAPConfigFieldServiceAccountPassword      = "serviceAccountPassword"
	OpenLDAPConfigFieldServiceAccountUsername      = "serviceAccountUsername"
	OpenLDAPConfigFieldTLS                         = "tls"
	OpenLDAPConfigFieldType                        = "type"
	OpenLDAPConfigFieldUserDisabledBitMask         = "userDisabledBitMask"
	OpenLDAPConfigFieldUserEnabledAttribute        = "userEnabledAttribute"
	OpenLDAPConfigFieldUserLoginAttribute          = "userLoginAttribute"
	OpenLDAPConfigFieldUserMemberAttribute         = "userMemberAttribute"
	OpenLDAPConfigFieldUserNameAttribute           = "userNameAttribute"
	OpenLDAPConfigFieldUserObjectClass             = "userObjectClass"
	OpenLDAPConfigFieldUserSearchAttribute         = "userSearchAttribute"
	OpenLDAPConfigFieldUserSearchBase              = "userSearchBase"
	OpenLDAPConfigFieldUuid                        = "uuid"
)

type OpenLDAPConfig struct {
	AccessMode                  string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowedPrincipalIDs         []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations                 map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Certificate                 string            `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ConnectionTimeout           int64             `json:"connectionTimeout,omitempty" yaml:"connectionTimeout,omitempty"`
	Created                     string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                   string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled                     bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GroupDNAttribute            string            `json:"groupDNAttribute,omitempty" yaml:"groupDNAttribute,omitempty"`
	GroupMemberMappingAttribute string            `json:"groupMemberMappingAttribute,omitempty" yaml:"groupMemberMappingAttribute,omitempty"`
	GroupMemberUserAttribute    string            `json:"groupMemberUserAttribute,omitempty" yaml:"groupMemberUserAttribute,omitempty"`
	GroupNameAttribute          string            `json:"groupNameAttribute,omitempty" yaml:"groupNameAttribute,omitempty"`
	GroupObjectClass            string            `json:"groupObjectClass,omitempty" yaml:"groupObjectClass,omitempty"`
	GroupSearchAttribute        string            `json:"groupSearchAttribute,omitempty" yaml:"groupSearchAttribute,omitempty"`
	GroupSearchBase             string            `json:"groupSearchBase,omitempty" yaml:"groupSearchBase,omitempty"`
	Labels                      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                        string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences             []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Port                        int64             `json:"port,omitempty" yaml:"port,omitempty"`
	Removed                     string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Servers                     []string          `json:"servers,omitempty" yaml:"servers,omitempty"`
	ServiceAccountPassword      string            `json:"serviceAccountPassword,omitempty" yaml:"serviceAccountPassword,omitempty"`
	ServiceAccountUsername      string            `json:"serviceAccountUsername,omitempty" yaml:"serviceAccountUsername,omitempty"`
	TLS                         bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	Type                        string            `json:"type,omitempty" yaml:"type,omitempty"`
	UserDisabledBitMask         int64             `json:"userDisabledBitMask,omitempty" yaml:"userDisabledBitMask,omitempty"`
	UserEnabledAttribute        string            `json:"userEnabledAttribute,omitempty" yaml:"userEnabledAttribute,omitempty"`
	UserLoginAttribute          string            `json:"userLoginAttribute,omitempty" yaml:"userLoginAttribute,omitempty"`
	UserMemberAttribute         string            `json:"userMemberAttribute,omitempty" yaml:"userMemberAttribute,omitempty"`
	UserNameAttribute           string            `json:"userNameAttribute,omitempty" yaml:"userNameAttribute,omitempty"`
	UserObjectClass             string            `json:"userObjectClass,omitempty" yaml:"userObjectClass,omitempty"`
	UserSearchAttribute         string            `json:"userSearchAttribute,omitempty" yaml:"userSearchAttribute,omitempty"`
	UserSearchBase              string            `json:"userSearchBase,omitempty" yaml:"userSearchBase,omitempty"`
	Uuid                        string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
