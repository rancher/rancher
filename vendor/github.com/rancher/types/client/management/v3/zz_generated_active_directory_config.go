package client

const (
	ActiveDirectoryConfigType                             = "activeDirectoryConfig"
	ActiveDirectoryConfigFieldAnnotations                 = "annotations"
	ActiveDirectoryConfigFieldConnectionTimeout           = "connectionTimeout"
	ActiveDirectoryConfigFieldCreated                     = "created"
	ActiveDirectoryConfigFieldCreatorID                   = "creatorId"
	ActiveDirectoryConfigFieldDomain                      = "domain"
	ActiveDirectoryConfigFieldEnabled                     = "enabled"
	ActiveDirectoryConfigFieldGroupDNField                = "groupDNField"
	ActiveDirectoryConfigFieldGroupMemberMappingAttribute = "groupMemberMappingAttribute"
	ActiveDirectoryConfigFieldGroupMemberUserAttribute    = "groupMemberUserAttribute"
	ActiveDirectoryConfigFieldGroupNameField              = "groupNameField"
	ActiveDirectoryConfigFieldGroupObjectClass            = "groupObjectClass"
	ActiveDirectoryConfigFieldGroupSearchDomain           = "groupSearchDomain"
	ActiveDirectoryConfigFieldGroupSearchField            = "groupSearchField"
	ActiveDirectoryConfigFieldLabels                      = "labels"
	ActiveDirectoryConfigFieldLoginDomain                 = "loginDomain"
	ActiveDirectoryConfigFieldName                        = "name"
	ActiveDirectoryConfigFieldOwnerReferences             = "ownerReferences"
	ActiveDirectoryConfigFieldPort                        = "port"
	ActiveDirectoryConfigFieldRemoved                     = "removed"
	ActiveDirectoryConfigFieldServer                      = "server"
	ActiveDirectoryConfigFieldServiceAccountPassword      = "serviceAccountPassword"
	ActiveDirectoryConfigFieldServiceAccountUsername      = "serviceAccountUsername"
	ActiveDirectoryConfigFieldTLS                         = "tls"
	ActiveDirectoryConfigFieldType                        = "type"
	ActiveDirectoryConfigFieldUserDisabledBitMask         = "userDisabledBitMask"
	ActiveDirectoryConfigFieldUserEnabledAttribute        = "userEnabledAttribute"
	ActiveDirectoryConfigFieldUserLoginField              = "userLoginField"
	ActiveDirectoryConfigFieldUserNameField               = "userNameField"
	ActiveDirectoryConfigFieldUserObjectClass             = "userObjectClass"
	ActiveDirectoryConfigFieldUserSearchField             = "userSearchField"
	ActiveDirectoryConfigFieldUuid                        = "uuid"
)

type ActiveDirectoryConfig struct {
	Annotations                 map[string]string `json:"annotations,omitempty"`
	ConnectionTimeout           *int64            `json:"connectionTimeout,omitempty"`
	Created                     string            `json:"created,omitempty"`
	CreatorID                   string            `json:"creatorId,omitempty"`
	Domain                      string            `json:"domain,omitempty"`
	Enabled                     *bool             `json:"enabled,omitempty"`
	GroupDNField                string            `json:"groupDNField,omitempty"`
	GroupMemberMappingAttribute string            `json:"groupMemberMappingAttribute,omitempty"`
	GroupMemberUserAttribute    string            `json:"groupMemberUserAttribute,omitempty"`
	GroupNameField              string            `json:"groupNameField,omitempty"`
	GroupObjectClass            string            `json:"groupObjectClass,omitempty"`
	GroupSearchDomain           string            `json:"groupSearchDomain,omitempty"`
	GroupSearchField            string            `json:"groupSearchField,omitempty"`
	Labels                      map[string]string `json:"labels,omitempty"`
	LoginDomain                 string            `json:"loginDomain,omitempty"`
	Name                        string            `json:"name,omitempty"`
	OwnerReferences             []OwnerReference  `json:"ownerReferences,omitempty"`
	Port                        *int64            `json:"port,omitempty"`
	Removed                     string            `json:"removed,omitempty"`
	Server                      string            `json:"server,omitempty"`
	ServiceAccountPassword      string            `json:"serviceAccountPassword,omitempty"`
	ServiceAccountUsername      string            `json:"serviceAccountUsername,omitempty"`
	TLS                         *bool             `json:"tls,omitempty"`
	Type                        string            `json:"type,omitempty"`
	UserDisabledBitMask         *int64            `json:"userDisabledBitMask,omitempty"`
	UserEnabledAttribute        string            `json:"userEnabledAttribute,omitempty"`
	UserLoginField              string            `json:"userLoginField,omitempty"`
	UserNameField               string            `json:"userNameField,omitempty"`
	UserObjectClass             string            `json:"userObjectClass,omitempty"`
	UserSearchField             string            `json:"userSearchField,omitempty"`
	Uuid                        string            `json:"uuid,omitempty"`
}
