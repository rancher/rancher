package client

const (
	FreeIpaConfigType                                 = "freeIpaConfig"
	FreeIpaConfigFieldAccessMode                      = "accessMode"
	FreeIpaConfigFieldAllowedPrincipalIDs             = "allowedPrincipalIds"
	FreeIpaConfigFieldAnnotations                     = "annotations"
	FreeIpaConfigFieldCertificate                     = "certificate"
	FreeIpaConfigFieldConnectionTimeout               = "connectionTimeout"
	FreeIpaConfigFieldCreated                         = "created"
	FreeIpaConfigFieldCreatorID                       = "creatorId"
	FreeIpaConfigFieldEnabled                         = "enabled"
	FreeIpaConfigFieldGroupDNAttribute                = "groupDNAttribute"
	FreeIpaConfigFieldGroupMemberMappingAttribute     = "groupMemberMappingAttribute"
	FreeIpaConfigFieldGroupMemberUserAttribute        = "groupMemberUserAttribute"
	FreeIpaConfigFieldGroupNameAttribute              = "groupNameAttribute"
	FreeIpaConfigFieldGroupObjectClass                = "groupObjectClass"
	FreeIpaConfigFieldGroupSearchAttribute            = "groupSearchAttribute"
	FreeIpaConfigFieldGroupSearchBase                 = "groupSearchBase"
	FreeIpaConfigFieldLabels                          = "labels"
	FreeIpaConfigFieldName                            = "name"
	FreeIpaConfigFieldOwnerReferences                 = "ownerReferences"
	FreeIpaConfigFieldPort                            = "port"
	FreeIpaConfigFieldRemoved                         = "removed"
	FreeIpaConfigFieldServers                         = "servers"
	FreeIpaConfigFieldServiceAccountDistinguishedName = "serviceAccountDistinguishedName"
	FreeIpaConfigFieldServiceAccountPassword          = "serviceAccountPassword"
	FreeIpaConfigFieldTLS                             = "tls"
	FreeIpaConfigFieldType                            = "type"
	FreeIpaConfigFieldUserDisabledBitMask             = "userDisabledBitMask"
	FreeIpaConfigFieldUserEnabledAttribute            = "userEnabledAttribute"
	FreeIpaConfigFieldUserLoginAttribute              = "userLoginAttribute"
	FreeIpaConfigFieldUserMemberAttribute             = "userMemberAttribute"
	FreeIpaConfigFieldUserNameAttribute               = "userNameAttribute"
	FreeIpaConfigFieldUserObjectClass                 = "userObjectClass"
	FreeIpaConfigFieldUserSearchAttribute             = "userSearchAttribute"
	FreeIpaConfigFieldUserSearchBase                  = "userSearchBase"
	FreeIpaConfigFieldUuid                            = "uuid"
)

type FreeIpaConfig struct {
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
	Labels                          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences                 []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Port                            int64             `json:"port,omitempty" yaml:"port,omitempty"`
	Removed                         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Servers                         []string          `json:"servers,omitempty" yaml:"servers,omitempty"`
	ServiceAccountDistinguishedName string            `json:"serviceAccountDistinguishedName,omitempty" yaml:"serviceAccountDistinguishedName,omitempty"`
	ServiceAccountPassword          string            `json:"serviceAccountPassword,omitempty" yaml:"serviceAccountPassword,omitempty"`
	TLS                             bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	Type                            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UserDisabledBitMask             int64             `json:"userDisabledBitMask,omitempty" yaml:"userDisabledBitMask,omitempty"`
	UserEnabledAttribute            string            `json:"userEnabledAttribute,omitempty" yaml:"userEnabledAttribute,omitempty"`
	UserLoginAttribute              string            `json:"userLoginAttribute,omitempty" yaml:"userLoginAttribute,omitempty"`
	UserMemberAttribute             string            `json:"userMemberAttribute,omitempty" yaml:"userMemberAttribute,omitempty"`
	UserNameAttribute               string            `json:"userNameAttribute,omitempty" yaml:"userNameAttribute,omitempty"`
	UserObjectClass                 string            `json:"userObjectClass,omitempty" yaml:"userObjectClass,omitempty"`
	UserSearchAttribute             string            `json:"userSearchAttribute,omitempty" yaml:"userSearchAttribute,omitempty"`
	UserSearchBase                  string            `json:"userSearchBase,omitempty" yaml:"userSearchBase,omitempty"`
	Uuid                            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
