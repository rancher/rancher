package client

import (
	"github.com/rancher/norman/types"
)

const (
	OpenLdapTestAndApplyInputType                                 = "openLdapTestAndApplyInput"
	OpenLdapTestAndApplyInputFieldAccessMode                      = "accessMode"
	OpenLdapTestAndApplyInputFieldAllowedPrincipalIDs             = "allowedPrincipalIds"
	OpenLdapTestAndApplyInputFieldAnnotations                     = "annotations"
	OpenLdapTestAndApplyInputFieldCertificate                     = "certificate"
	OpenLdapTestAndApplyInputFieldConnectionTimeout               = "connectionTimeout"
	OpenLdapTestAndApplyInputFieldCreated                         = "created"
	OpenLdapTestAndApplyInputFieldCreatorID                       = "creatorId"
	OpenLdapTestAndApplyInputFieldEnabled                         = "enabled"
	OpenLdapTestAndApplyInputFieldGroupDNAttribute                = "groupDNAttribute"
	OpenLdapTestAndApplyInputFieldGroupMemberMappingAttribute     = "groupMemberMappingAttribute"
	OpenLdapTestAndApplyInputFieldGroupMemberUserAttribute        = "groupMemberUserAttribute"
	OpenLdapTestAndApplyInputFieldGroupNameAttribute              = "groupNameAttribute"
	OpenLdapTestAndApplyInputFieldGroupObjectClass                = "groupObjectClass"
	OpenLdapTestAndApplyInputFieldGroupSearchAttribute            = "groupSearchAttribute"
	OpenLdapTestAndApplyInputFieldGroupSearchBase                 = "groupSearchBase"
	OpenLdapTestAndApplyInputFieldLabels                          = "labels"
	OpenLdapTestAndApplyInputFieldName                            = "name"
	OpenLdapTestAndApplyInputFieldOwnerReferences                 = "ownerReferences"
	OpenLdapTestAndApplyInputFieldPassword                        = "password"
	OpenLdapTestAndApplyInputFieldPort                            = "port"
	OpenLdapTestAndApplyInputFieldRemoved                         = "removed"
	OpenLdapTestAndApplyInputFieldServers                         = "servers"
	OpenLdapTestAndApplyInputFieldServiceAccountDistinguishedName = "serviceAccountDistinguishedName"
	OpenLdapTestAndApplyInputFieldServiceAccountPassword          = "serviceAccountPassword"
	OpenLdapTestAndApplyInputFieldTLS                             = "tls"
	OpenLdapTestAndApplyInputFieldType                            = "type"
	OpenLdapTestAndApplyInputFieldUserDisabledBitMask             = "userDisabledBitMask"
	OpenLdapTestAndApplyInputFieldUserEnabledAttribute            = "userEnabledAttribute"
	OpenLdapTestAndApplyInputFieldUserLoginAttribute              = "userLoginAttribute"
	OpenLdapTestAndApplyInputFieldUserMemberAttribute             = "userMemberAttribute"
	OpenLdapTestAndApplyInputFieldUserNameAttribute               = "userNameAttribute"
	OpenLdapTestAndApplyInputFieldUserObjectClass                 = "userObjectClass"
	OpenLdapTestAndApplyInputFieldUserSearchAttribute             = "userSearchAttribute"
	OpenLdapTestAndApplyInputFieldUserSearchBase                  = "userSearchBase"
	OpenLdapTestAndApplyInputFieldUsername                        = "username"
	OpenLdapTestAndApplyInputFieldUuid                            = "uuid"
)

type OpenLdapTestAndApplyInput struct {
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
	Labels                          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences                 []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Password                        string            `json:"password,omitempty" yaml:"password,omitempty"`
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
	Username                        string            `json:"username,omitempty" yaml:"username,omitempty"`
	Uuid                            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type OpenLdapTestAndApplyInputCollection struct {
	types.Collection
	Data   []OpenLdapTestAndApplyInput `json:"data,omitempty"`
	client *OpenLdapTestAndApplyInputClient
}

type OpenLdapTestAndApplyInputClient struct {
	apiClient *Client
}

type OpenLdapTestAndApplyInputOperations interface {
	List(opts *types.ListOpts) (*OpenLdapTestAndApplyInputCollection, error)
	Create(opts *OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error)
	Update(existing *OpenLdapTestAndApplyInput, updates interface{}) (*OpenLdapTestAndApplyInput, error)
	ByID(id string) (*OpenLdapTestAndApplyInput, error)
	Delete(container *OpenLdapTestAndApplyInput) error
}

func newOpenLdapTestAndApplyInputClient(apiClient *Client) *OpenLdapTestAndApplyInputClient {
	return &OpenLdapTestAndApplyInputClient{
		apiClient: apiClient,
	}
}

func (c *OpenLdapTestAndApplyInputClient) Create(container *OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error) {
	resp := &OpenLdapTestAndApplyInput{}
	err := c.apiClient.Ops.DoCreate(OpenLdapTestAndApplyInputType, container, resp)
	return resp, err
}

func (c *OpenLdapTestAndApplyInputClient) Update(existing *OpenLdapTestAndApplyInput, updates interface{}) (*OpenLdapTestAndApplyInput, error) {
	resp := &OpenLdapTestAndApplyInput{}
	err := c.apiClient.Ops.DoUpdate(OpenLdapTestAndApplyInputType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *OpenLdapTestAndApplyInputClient) List(opts *types.ListOpts) (*OpenLdapTestAndApplyInputCollection, error) {
	resp := &OpenLdapTestAndApplyInputCollection{}
	err := c.apiClient.Ops.DoList(OpenLdapTestAndApplyInputType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *OpenLdapTestAndApplyInputCollection) Next() (*OpenLdapTestAndApplyInputCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &OpenLdapTestAndApplyInputCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *OpenLdapTestAndApplyInputClient) ByID(id string) (*OpenLdapTestAndApplyInput, error) {
	resp := &OpenLdapTestAndApplyInput{}
	err := c.apiClient.Ops.DoByID(OpenLdapTestAndApplyInputType, id, resp)
	return resp, err
}

func (c *OpenLdapTestAndApplyInputClient) Delete(container *OpenLdapTestAndApplyInput) error {
	return c.apiClient.Ops.DoResourceDelete(OpenLdapTestAndApplyInputType, &container.Resource)
}
