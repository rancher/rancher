package client

const (
	ShibbolethConfigType                                 = "shibbolethConfig"
	ShibbolethConfigFieldAccessMode                      = "accessMode"
	ShibbolethConfigFieldAllowedPrincipalIDs             = "allowedPrincipalIds"
	ShibbolethConfigFieldAnnotations                     = "annotations"
	ShibbolethConfigFieldCertificate                     = "certificate"
	ShibbolethConfigFieldConnectionTimeout               = "connectionTimeout"
	ShibbolethConfigFieldCreated                         = "created"
	ShibbolethConfigFieldCreatorID                       = "creatorId"
	ShibbolethConfigFieldDisplayNameField                = "displayNameField"
	ShibbolethConfigFieldEnabled                         = "enabled"
	ShibbolethConfigFieldGroupDNAttribute                = "groupDNAttribute"
	ShibbolethConfigFieldGroupMemberMappingAttribute     = "groupMemberMappingAttribute"
	ShibbolethConfigFieldGroupMemberUserAttribute        = "groupMemberUserAttribute"
	ShibbolethConfigFieldGroupNameAttribute              = "groupNameAttribute"
	ShibbolethConfigFieldGroupObjectClass                = "groupObjectClass"
	ShibbolethConfigFieldGroupSearchAttribute            = "groupSearchAttribute"
	ShibbolethConfigFieldGroupSearchBase                 = "groupSearchBase"
	ShibbolethConfigFieldGroupSearchFilter               = "groupSearchFilter"
	ShibbolethConfigFieldGroupsField                     = "groupsField"
	ShibbolethConfigFieldIDPMetadataContent              = "idpMetadataContent"
	ShibbolethConfigFieldLabels                          = "labels"
	ShibbolethConfigFieldName                            = "name"
	ShibbolethConfigFieldNestedGroupMembershipEnabled    = "nestedGroupMembershipEnabled"
	ShibbolethConfigFieldOwnerReferences                 = "ownerReferences"
	ShibbolethConfigFieldPort                            = "port"
	ShibbolethConfigFieldRancherAPIHost                  = "rancherApiHost"
	ShibbolethConfigFieldRemoved                         = "removed"
	ShibbolethConfigFieldServers                         = "servers"
	ShibbolethConfigFieldServiceAccountDistinguishedName = "serviceAccountDistinguishedName"
	ShibbolethConfigFieldServiceAccountPassword          = "serviceAccountPassword"
	ShibbolethConfigFieldSpCert                          = "spCert"
	ShibbolethConfigFieldSpKey                           = "spKey"
	ShibbolethConfigFieldTLS                             = "tls"
	ShibbolethConfigFieldType                            = "type"
	ShibbolethConfigFieldUIDField                        = "uidField"
	ShibbolethConfigFieldUUID                            = "uuid"
	ShibbolethConfigFieldUserDisabledBitMask             = "userDisabledBitMask"
	ShibbolethConfigFieldUserEnabledAttribute            = "userEnabledAttribute"
	ShibbolethConfigFieldUserLoginAttribute              = "userLoginAttribute"
	ShibbolethConfigFieldUserMemberAttribute             = "userMemberAttribute"
	ShibbolethConfigFieldUserNameAttribute               = "userNameAttribute"
	ShibbolethConfigFieldUserNameField                   = "userNameField"
	ShibbolethConfigFieldUserObjectClass                 = "userObjectClass"
	ShibbolethConfigFieldUserSearchAttribute             = "userSearchAttribute"
	ShibbolethConfigFieldUserSearchBase                  = "userSearchBase"
	ShibbolethConfigFieldUserSearchFilter                = "userSearchFilter"
)

type ShibbolethConfig struct {
	AccessMode                      string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowedPrincipalIDs             []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations                     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Certificate                     string            `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ConnectionTimeout               int64             `json:"connectionTimeout,omitempty" yaml:"connectionTimeout,omitempty"`
	Created                         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DisplayNameField                string            `json:"displayNameField,omitempty" yaml:"displayNameField,omitempty"`
	Enabled                         bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GroupDNAttribute                string            `json:"groupDNAttribute,omitempty" yaml:"groupDNAttribute,omitempty"`
	GroupMemberMappingAttribute     string            `json:"groupMemberMappingAttribute,omitempty" yaml:"groupMemberMappingAttribute,omitempty"`
	GroupMemberUserAttribute        string            `json:"groupMemberUserAttribute,omitempty" yaml:"groupMemberUserAttribute,omitempty"`
	GroupNameAttribute              string            `json:"groupNameAttribute,omitempty" yaml:"groupNameAttribute,omitempty"`
	GroupObjectClass                string            `json:"groupObjectClass,omitempty" yaml:"groupObjectClass,omitempty"`
	GroupSearchAttribute            string            `json:"groupSearchAttribute,omitempty" yaml:"groupSearchAttribute,omitempty"`
	GroupSearchBase                 string            `json:"groupSearchBase,omitempty" yaml:"groupSearchBase,omitempty"`
	GroupSearchFilter               string            `json:"groupSearchFilter,omitempty" yaml:"groupSearchFilter,omitempty"`
	GroupsField                     string            `json:"groupsField,omitempty" yaml:"groupsField,omitempty"`
	IDPMetadataContent              string            `json:"idpMetadataContent,omitempty" yaml:"idpMetadataContent,omitempty"`
	Labels                          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NestedGroupMembershipEnabled    bool              `json:"nestedGroupMembershipEnabled,omitempty" yaml:"nestedGroupMembershipEnabled,omitempty"`
	OwnerReferences                 []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Port                            int64             `json:"port,omitempty" yaml:"port,omitempty"`
	RancherAPIHost                  string            `json:"rancherApiHost,omitempty" yaml:"rancherApiHost,omitempty"`
	Removed                         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Servers                         []string          `json:"servers,omitempty" yaml:"servers,omitempty"`
	ServiceAccountDistinguishedName string            `json:"serviceAccountDistinguishedName,omitempty" yaml:"serviceAccountDistinguishedName,omitempty"`
	ServiceAccountPassword          string            `json:"serviceAccountPassword,omitempty" yaml:"serviceAccountPassword,omitempty"`
	SpCert                          string            `json:"spCert,omitempty" yaml:"spCert,omitempty"`
	SpKey                           string            `json:"spKey,omitempty" yaml:"spKey,omitempty"`
	TLS                             bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	Type                            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UIDField                        string            `json:"uidField,omitempty" yaml:"uidField,omitempty"`
	UUID                            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserDisabledBitMask             int64             `json:"userDisabledBitMask,omitempty" yaml:"userDisabledBitMask,omitempty"`
	UserEnabledAttribute            string            `json:"userEnabledAttribute,omitempty" yaml:"userEnabledAttribute,omitempty"`
	UserLoginAttribute              string            `json:"userLoginAttribute,omitempty" yaml:"userLoginAttribute,omitempty"`
	UserMemberAttribute             string            `json:"userMemberAttribute,omitempty" yaml:"userMemberAttribute,omitempty"`
	UserNameAttribute               string            `json:"userNameAttribute,omitempty" yaml:"userNameAttribute,omitempty"`
	UserNameField                   string            `json:"userNameField,omitempty" yaml:"userNameField,omitempty"`
	UserObjectClass                 string            `json:"userObjectClass,omitempty" yaml:"userObjectClass,omitempty"`
	UserSearchAttribute             string            `json:"userSearchAttribute,omitempty" yaml:"userSearchAttribute,omitempty"`
	UserSearchBase                  string            `json:"userSearchBase,omitempty" yaml:"userSearchBase,omitempty"`
	UserSearchFilter                string            `json:"userSearchFilter,omitempty" yaml:"userSearchFilter,omitempty"`
}
