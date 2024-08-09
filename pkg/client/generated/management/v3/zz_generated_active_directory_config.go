package client

const (
	ActiveDirectoryConfigType                              = "activeDirectoryConfig"
	ActiveDirectoryConfigFieldAccessMode                   = "accessMode"
	ActiveDirectoryConfigFieldAllowedPrincipalIDs          = "allowedPrincipalIds"
	ActiveDirectoryConfigFieldAnnotations                  = "annotations"
	ActiveDirectoryConfigFieldCertificate                  = "certificate"
	ActiveDirectoryConfigFieldConnectionTimeout            = "connectionTimeout"
	ActiveDirectoryConfigFieldCreated                      = "created"
	ActiveDirectoryConfigFieldCreatorID                    = "creatorId"
	ActiveDirectoryConfigFieldDefaultLoginDomain           = "defaultLoginDomain"
	ActiveDirectoryConfigFieldEnabled                      = "enabled"
	ActiveDirectoryConfigFieldGroupDNAttribute             = "groupDNAttribute"
	ActiveDirectoryConfigFieldGroupMemberMappingAttribute  = "groupMemberMappingAttribute"
	ActiveDirectoryConfigFieldGroupMemberUserAttribute     = "groupMemberUserAttribute"
	ActiveDirectoryConfigFieldGroupNameAttribute           = "groupNameAttribute"
	ActiveDirectoryConfigFieldGroupObjectClass             = "groupObjectClass"
	ActiveDirectoryConfigFieldGroupSearchAttribute         = "groupSearchAttribute"
	ActiveDirectoryConfigFieldGroupSearchBase              = "groupSearchBase"
	ActiveDirectoryConfigFieldGroupSearchFilter            = "groupSearchFilter"
	ActiveDirectoryConfigFieldLabels                       = "labels"
	ActiveDirectoryConfigFieldLogoutAllSupported           = "logoutAllSupported"
	ActiveDirectoryConfigFieldName                         = "name"
	ActiveDirectoryConfigFieldNestedGroupMembershipEnabled = "nestedGroupMembershipEnabled"
	ActiveDirectoryConfigFieldOwnerReferences              = "ownerReferences"
	ActiveDirectoryConfigFieldPort                         = "port"
	ActiveDirectoryConfigFieldRemoved                      = "removed"
	ActiveDirectoryConfigFieldServers                      = "servers"
	ActiveDirectoryConfigFieldServiceAccountPassword       = "serviceAccountPassword"
	ActiveDirectoryConfigFieldServiceAccountUsername       = "serviceAccountUsername"
	ActiveDirectoryConfigFieldStartTLS                     = "starttls"
	ActiveDirectoryConfigFieldStatus                       = "status"
	ActiveDirectoryConfigFieldTLS                          = "tls"
	ActiveDirectoryConfigFieldType                         = "type"
	ActiveDirectoryConfigFieldUUID                         = "uuid"
	ActiveDirectoryConfigFieldUserDisabledBitMask          = "userDisabledBitMask"
	ActiveDirectoryConfigFieldUserEnabledAttribute         = "userEnabledAttribute"
	ActiveDirectoryConfigFieldUserLoginAttribute           = "userLoginAttribute"
	ActiveDirectoryConfigFieldUserNameAttribute            = "userNameAttribute"
	ActiveDirectoryConfigFieldUserObjectClass              = "userObjectClass"
	ActiveDirectoryConfigFieldUserSearchAttribute          = "userSearchAttribute"
	ActiveDirectoryConfigFieldUserSearchBase               = "userSearchBase"
	ActiveDirectoryConfigFieldUserSearchFilter             = "userSearchFilter"
)

type ActiveDirectoryConfig struct {
	AccessMode                   string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowedPrincipalIDs          []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations                  map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Certificate                  string            `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ConnectionTimeout            int64             `json:"connectionTimeout,omitempty" yaml:"connectionTimeout,omitempty"`
	Created                      string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                    string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DefaultLoginDomain           string            `json:"defaultLoginDomain,omitempty" yaml:"defaultLoginDomain,omitempty"`
	Enabled                      bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GroupDNAttribute             string            `json:"groupDNAttribute,omitempty" yaml:"groupDNAttribute,omitempty"`
	GroupMemberMappingAttribute  string            `json:"groupMemberMappingAttribute,omitempty" yaml:"groupMemberMappingAttribute,omitempty"`
	GroupMemberUserAttribute     string            `json:"groupMemberUserAttribute,omitempty" yaml:"groupMemberUserAttribute,omitempty"`
	GroupNameAttribute           string            `json:"groupNameAttribute,omitempty" yaml:"groupNameAttribute,omitempty"`
	GroupObjectClass             string            `json:"groupObjectClass,omitempty" yaml:"groupObjectClass,omitempty"`
	GroupSearchAttribute         string            `json:"groupSearchAttribute,omitempty" yaml:"groupSearchAttribute,omitempty"`
	GroupSearchBase              string            `json:"groupSearchBase,omitempty" yaml:"groupSearchBase,omitempty"`
	GroupSearchFilter            string            `json:"groupSearchFilter,omitempty" yaml:"groupSearchFilter,omitempty"`
	Labels                       map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LogoutAllSupported           bool              `json:"logoutAllSupported,omitempty" yaml:"logoutAllSupported,omitempty"`
	Name                         string            `json:"name,omitempty" yaml:"name,omitempty"`
	NestedGroupMembershipEnabled *bool             `json:"nestedGroupMembershipEnabled,omitempty" yaml:"nestedGroupMembershipEnabled,omitempty"`
	OwnerReferences              []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Port                         int64             `json:"port,omitempty" yaml:"port,omitempty"`
	Removed                      string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Servers                      []string          `json:"servers,omitempty" yaml:"servers,omitempty"`
	ServiceAccountPassword       string            `json:"serviceAccountPassword,omitempty" yaml:"serviceAccountPassword,omitempty"`
	ServiceAccountUsername       string            `json:"serviceAccountUsername,omitempty" yaml:"serviceAccountUsername,omitempty"`
	StartTLS                     bool              `json:"starttls,omitempty" yaml:"starttls,omitempty"`
	Status                       *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	TLS                          bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	Type                         string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                         string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserDisabledBitMask          int64             `json:"userDisabledBitMask,omitempty" yaml:"userDisabledBitMask,omitempty"`
	UserEnabledAttribute         string            `json:"userEnabledAttribute,omitempty" yaml:"userEnabledAttribute,omitempty"`
	UserLoginAttribute           string            `json:"userLoginAttribute,omitempty" yaml:"userLoginAttribute,omitempty"`
	UserNameAttribute            string            `json:"userNameAttribute,omitempty" yaml:"userNameAttribute,omitempty"`
	UserObjectClass              string            `json:"userObjectClass,omitempty" yaml:"userObjectClass,omitempty"`
	UserSearchAttribute          string            `json:"userSearchAttribute,omitempty" yaml:"userSearchAttribute,omitempty"`
	UserSearchBase               string            `json:"userSearchBase,omitempty" yaml:"userSearchBase,omitempty"`
	UserSearchFilter             string            `json:"userSearchFilter,omitempty" yaml:"userSearchFilter,omitempty"`
}
