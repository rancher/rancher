package client

const (
	ActiveDirectoryConfigType                             = "activeDirectoryConfig"
	ActiveDirectoryConfigFieldAccessMode                  = "accessMode"
	ActiveDirectoryConfigFieldAllowedPrincipalIDs         = "allowedPrincipalIds"
	ActiveDirectoryConfigFieldAnnotations                 = "annotations"
	ActiveDirectoryConfigFieldCertificate                 = "certificate"
	ActiveDirectoryConfigFieldConnectionTimeout           = "connectionTimeout"
	ActiveDirectoryConfigFieldCreated                     = "created"
	ActiveDirectoryConfigFieldCreatorID                   = "creatorId"
	ActiveDirectoryConfigFieldDefaultLoginDomain          = "defaultLoginDomain"
	ActiveDirectoryConfigFieldEnabled                     = "enabled"
	ActiveDirectoryConfigFieldGroupDNAttribute            = "groupDNAttribute"
	ActiveDirectoryConfigFieldGroupMemberMappingAttribute = "groupMemberMappingAttribute"
	ActiveDirectoryConfigFieldGroupMemberUserAttribute    = "groupMemberUserAttribute"
	ActiveDirectoryConfigFieldGroupNameAttribute          = "groupNameAttribute"
	ActiveDirectoryConfigFieldGroupObjectClass            = "groupObjectClass"
	ActiveDirectoryConfigFieldGroupSearchAttribute        = "groupSearchAttribute"
	ActiveDirectoryConfigFieldGroupSearchBase             = "groupSearchBase"
	ActiveDirectoryConfigFieldLabels                      = "labels"
	ActiveDirectoryConfigFieldName                        = "name"
	ActiveDirectoryConfigFieldOwnerReferences             = "ownerReferences"
	ActiveDirectoryConfigFieldPort                        = "port"
	ActiveDirectoryConfigFieldRemoved                     = "removed"
	ActiveDirectoryConfigFieldServers                     = "servers"
	ActiveDirectoryConfigFieldServiceAccountPassword      = "serviceAccountPassword"
	ActiveDirectoryConfigFieldServiceAccountUsername      = "serviceAccountUsername"
	ActiveDirectoryConfigFieldTLS                         = "tls"
	ActiveDirectoryConfigFieldType                        = "type"
	ActiveDirectoryConfigFieldUserDisabledBitMask         = "userDisabledBitMask"
	ActiveDirectoryConfigFieldUserEnabledAttribute        = "userEnabledAttribute"
	ActiveDirectoryConfigFieldUserLoginAttribute          = "userLoginAttribute"
	ActiveDirectoryConfigFieldUserNameAttribute           = "userNameAttribute"
	ActiveDirectoryConfigFieldUserObjectClass             = "userObjectClass"
	ActiveDirectoryConfigFieldUserSearchAttribute         = "userSearchAttribute"
	ActiveDirectoryConfigFieldUserSearchBase              = "userSearchBase"
	ActiveDirectoryConfigFieldUuid                        = "uuid"
)

type ActiveDirectoryConfig struct {
	AccessMode                  string            `json:"accessMode,omitempty"`
	AllowedPrincipalIDs         []string          `json:"allowedPrincipalIds,omitempty"`
	Annotations                 map[string]string `json:"annotations,omitempty"`
	Certificate                 string            `json:"certificate,omitempty"`
	ConnectionTimeout           *int64            `json:"connectionTimeout,omitempty"`
	Created                     string            `json:"created,omitempty"`
	CreatorID                   string            `json:"creatorId,omitempty"`
	DefaultLoginDomain          string            `json:"defaultLoginDomain,omitempty"`
	Enabled                     bool              `json:"enabled,omitempty"`
	GroupDNAttribute            string            `json:"groupDNAttribute,omitempty"`
	GroupMemberMappingAttribute string            `json:"groupMemberMappingAttribute,omitempty"`
	GroupMemberUserAttribute    string            `json:"groupMemberUserAttribute,omitempty"`
	GroupNameAttribute          string            `json:"groupNameAttribute,omitempty"`
	GroupObjectClass            string            `json:"groupObjectClass,omitempty"`
	GroupSearchAttribute        string            `json:"groupSearchAttribute,omitempty"`
	GroupSearchBase             string            `json:"groupSearchBase,omitempty"`
	Labels                      map[string]string `json:"labels,omitempty"`
	Name                        string            `json:"name,omitempty"`
	OwnerReferences             []OwnerReference  `json:"ownerReferences,omitempty"`
	Port                        *int64            `json:"port,omitempty"`
	Removed                     string            `json:"removed,omitempty"`
	Servers                     []string          `json:"servers,omitempty"`
	ServiceAccountPassword      string            `json:"serviceAccountPassword,omitempty"`
	ServiceAccountUsername      string            `json:"serviceAccountUsername,omitempty"`
	TLS                         bool              `json:"tls,omitempty"`
	Type                        string            `json:"type,omitempty"`
	UserDisabledBitMask         *int64            `json:"userDisabledBitMask,omitempty"`
	UserEnabledAttribute        string            `json:"userEnabledAttribute,omitempty"`
	UserLoginAttribute          string            `json:"userLoginAttribute,omitempty"`
	UserNameAttribute           string            `json:"userNameAttribute,omitempty"`
	UserObjectClass             string            `json:"userObjectClass,omitempty"`
	UserSearchAttribute         string            `json:"userSearchAttribute,omitempty"`
	UserSearchBase              string            `json:"userSearchBase,omitempty"`
	Uuid                        string            `json:"uuid,omitempty"`
}
