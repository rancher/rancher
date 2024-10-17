package client

import (
	"github.com/rancher/norman/types"
)

const (
	LdapConfigType                                 = "ldapConfig"
	LdapConfigFieldAccessMode                      = "accessMode"
	LdapConfigFieldAllowedPrincipalIDs             = "allowedPrincipalIds"
	LdapConfigFieldAnnotations                     = "annotations"
	LdapConfigFieldCertificate                     = "certificate"
	LdapConfigFieldConnectionTimeout               = "connectionTimeout"
	LdapConfigFieldCreated                         = "created"
	LdapConfigFieldCreatorID                       = "creatorId"
	LdapConfigFieldEnabled                         = "enabled"
	LdapConfigFieldGroupDNAttribute                = "groupDNAttribute"
	LdapConfigFieldGroupMemberMappingAttribute     = "groupMemberMappingAttribute"
	LdapConfigFieldGroupMemberUserAttribute        = "groupMemberUserAttribute"
	LdapConfigFieldGroupNameAttribute              = "groupNameAttribute"
	LdapConfigFieldGroupObjectClass                = "groupObjectClass"
	LdapConfigFieldGroupSearchAttribute            = "groupSearchAttribute"
	LdapConfigFieldGroupSearchBase                 = "groupSearchBase"
	LdapConfigFieldGroupSearchFilter               = "groupSearchFilter"
	LdapConfigFieldLabels                          = "labels"
	LdapConfigFieldLogoutAllSupported              = "logoutAllSupported"
	LdapConfigFieldName                            = "name"
	LdapConfigFieldNestedGroupMembershipEnabled    = "nestedGroupMembershipEnabled"
	LdapConfigFieldOwnerReferences                 = "ownerReferences"
	LdapConfigFieldPort                            = "port"
	LdapConfigFieldRemoved                         = "removed"
	LdapConfigFieldSearchUsingServiceAccount       = "searchUsingServiceAccount"
	LdapConfigFieldServers                         = "servers"
	LdapConfigFieldServiceAccountDistinguishedName = "serviceAccountDistinguishedName"
	LdapConfigFieldServiceAccountPassword          = "serviceAccountPassword"
	LdapConfigFieldStartTLS                        = "starttls"
	LdapConfigFieldStatus                          = "status"
	LdapConfigFieldTLS                             = "tls"
	LdapConfigFieldType                            = "type"
	LdapConfigFieldUUID                            = "uuid"
	LdapConfigFieldUserDisabledBitMask             = "userDisabledBitMask"
	LdapConfigFieldUserEnabledAttribute            = "userEnabledAttribute"
	LdapConfigFieldUserLoginAttribute              = "userLoginAttribute"
	LdapConfigFieldUserMemberAttribute             = "userMemberAttribute"
	LdapConfigFieldUserNameAttribute               = "userNameAttribute"
	LdapConfigFieldUserObjectClass                 = "userObjectClass"
	LdapConfigFieldUserSearchAttribute             = "userSearchAttribute"
	LdapConfigFieldUserSearchBase                  = "userSearchBase"
	LdapConfigFieldUserSearchFilter                = "userSearchFilter"
)

type LdapConfig struct {
	types.Resource
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

type LdapConfigCollection struct {
	types.Collection
	Data   []LdapConfig `json:"data,omitempty"`
	client *LdapConfigClient
}

type LdapConfigClient struct {
	apiClient *Client
}

type LdapConfigOperations interface {
	List(opts *types.ListOpts) (*LdapConfigCollection, error)
	ListAll(opts *types.ListOpts) (*LdapConfigCollection, error)
	Create(opts *LdapConfig) (*LdapConfig, error)
	Update(existing *LdapConfig, updates interface{}) (*LdapConfig, error)
	Replace(existing *LdapConfig) (*LdapConfig, error)
	ByID(id string) (*LdapConfig, error)
	Delete(container *LdapConfig) error
}

func newLdapConfigClient(apiClient *Client) *LdapConfigClient {
	return &LdapConfigClient{
		apiClient: apiClient,
	}
}

func (c *LdapConfigClient) Create(container *LdapConfig) (*LdapConfig, error) {
	resp := &LdapConfig{}
	err := c.apiClient.Ops.DoCreate(LdapConfigType, container, resp)
	return resp, err
}

func (c *LdapConfigClient) Update(existing *LdapConfig, updates interface{}) (*LdapConfig, error) {
	resp := &LdapConfig{}
	err := c.apiClient.Ops.DoUpdate(LdapConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *LdapConfigClient) Replace(obj *LdapConfig) (*LdapConfig, error) {
	resp := &LdapConfig{}
	err := c.apiClient.Ops.DoReplace(LdapConfigType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *LdapConfigClient) List(opts *types.ListOpts) (*LdapConfigCollection, error) {
	resp := &LdapConfigCollection{}
	err := c.apiClient.Ops.DoList(LdapConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *LdapConfigClient) ListAll(opts *types.ListOpts) (*LdapConfigCollection, error) {
	resp := &LdapConfigCollection{}
	resp, err := c.List(opts)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (cc *LdapConfigCollection) Next() (*LdapConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &LdapConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *LdapConfigClient) ByID(id string) (*LdapConfig, error) {
	resp := &LdapConfig{}
	err := c.apiClient.Ops.DoByID(LdapConfigType, id, resp)
	return resp, err
}

func (c *LdapConfigClient) Delete(container *LdapConfig) error {
	return c.apiClient.Ops.DoResourceDelete(LdapConfigType, &container.Resource)
}
