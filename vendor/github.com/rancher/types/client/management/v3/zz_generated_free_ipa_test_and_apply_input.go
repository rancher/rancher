package client

import (
	"github.com/rancher/norman/types"
)

const (
	FreeIpaTestAndApplyInputType                                 = "freeIpaTestAndApplyInput"
	FreeIpaTestAndApplyInputFieldAccessMode                      = "accessMode"
	FreeIpaTestAndApplyInputFieldAllowedPrincipalIDs             = "allowedPrincipalIds"
	FreeIpaTestAndApplyInputFieldAnnotations                     = "annotations"
	FreeIpaTestAndApplyInputFieldCertificate                     = "certificate"
	FreeIpaTestAndApplyInputFieldConnectionTimeout               = "connectionTimeout"
	FreeIpaTestAndApplyInputFieldCreated                         = "created"
	FreeIpaTestAndApplyInputFieldCreatorID                       = "creatorId"
	FreeIpaTestAndApplyInputFieldEnabled                         = "enabled"
	FreeIpaTestAndApplyInputFieldGroupDNAttribute                = "groupDNAttribute"
	FreeIpaTestAndApplyInputFieldGroupMemberMappingAttribute     = "groupMemberMappingAttribute"
	FreeIpaTestAndApplyInputFieldGroupMemberUserAttribute        = "groupMemberUserAttribute"
	FreeIpaTestAndApplyInputFieldGroupNameAttribute              = "groupNameAttribute"
	FreeIpaTestAndApplyInputFieldGroupObjectClass                = "groupObjectClass"
	FreeIpaTestAndApplyInputFieldGroupSearchAttribute            = "groupSearchAttribute"
	FreeIpaTestAndApplyInputFieldGroupSearchBase                 = "groupSearchBase"
	FreeIpaTestAndApplyInputFieldLabels                          = "labels"
	FreeIpaTestAndApplyInputFieldName                            = "name"
	FreeIpaTestAndApplyInputFieldOwnerReferences                 = "ownerReferences"
	FreeIpaTestAndApplyInputFieldPassword                        = "password"
	FreeIpaTestAndApplyInputFieldPort                            = "port"
	FreeIpaTestAndApplyInputFieldRemoved                         = "removed"
	FreeIpaTestAndApplyInputFieldServers                         = "servers"
	FreeIpaTestAndApplyInputFieldServiceAccountDistinguishedName = "serviceAccountDistinguishedName"
	FreeIpaTestAndApplyInputFieldServiceAccountPassword          = "serviceAccountPassword"
	FreeIpaTestAndApplyInputFieldTLS                             = "tls"
	FreeIpaTestAndApplyInputFieldType                            = "type"
	FreeIpaTestAndApplyInputFieldUserDisabledBitMask             = "userDisabledBitMask"
	FreeIpaTestAndApplyInputFieldUserEnabledAttribute            = "userEnabledAttribute"
	FreeIpaTestAndApplyInputFieldUserLoginAttribute              = "userLoginAttribute"
	FreeIpaTestAndApplyInputFieldUserMemberAttribute             = "userMemberAttribute"
	FreeIpaTestAndApplyInputFieldUserNameAttribute               = "userNameAttribute"
	FreeIpaTestAndApplyInputFieldUserObjectClass                 = "userObjectClass"
	FreeIpaTestAndApplyInputFieldUserSearchAttribute             = "userSearchAttribute"
	FreeIpaTestAndApplyInputFieldUserSearchBase                  = "userSearchBase"
	FreeIpaTestAndApplyInputFieldUsername                        = "username"
	FreeIpaTestAndApplyInputFieldUuid                            = "uuid"
)

type FreeIpaTestAndApplyInput struct {
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
type FreeIpaTestAndApplyInputCollection struct {
	types.Collection
	Data   []FreeIpaTestAndApplyInput `json:"data,omitempty"`
	client *FreeIpaTestAndApplyInputClient
}

type FreeIpaTestAndApplyInputClient struct {
	apiClient *Client
}

type FreeIpaTestAndApplyInputOperations interface {
	List(opts *types.ListOpts) (*FreeIpaTestAndApplyInputCollection, error)
	Create(opts *FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error)
	Update(existing *FreeIpaTestAndApplyInput, updates interface{}) (*FreeIpaTestAndApplyInput, error)
	ByID(id string) (*FreeIpaTestAndApplyInput, error)
	Delete(container *FreeIpaTestAndApplyInput) error
}

func newFreeIpaTestAndApplyInputClient(apiClient *Client) *FreeIpaTestAndApplyInputClient {
	return &FreeIpaTestAndApplyInputClient{
		apiClient: apiClient,
	}
}

func (c *FreeIpaTestAndApplyInputClient) Create(container *FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error) {
	resp := &FreeIpaTestAndApplyInput{}
	err := c.apiClient.Ops.DoCreate(FreeIpaTestAndApplyInputType, container, resp)
	return resp, err
}

func (c *FreeIpaTestAndApplyInputClient) Update(existing *FreeIpaTestAndApplyInput, updates interface{}) (*FreeIpaTestAndApplyInput, error) {
	resp := &FreeIpaTestAndApplyInput{}
	err := c.apiClient.Ops.DoUpdate(FreeIpaTestAndApplyInputType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *FreeIpaTestAndApplyInputClient) List(opts *types.ListOpts) (*FreeIpaTestAndApplyInputCollection, error) {
	resp := &FreeIpaTestAndApplyInputCollection{}
	err := c.apiClient.Ops.DoList(FreeIpaTestAndApplyInputType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *FreeIpaTestAndApplyInputCollection) Next() (*FreeIpaTestAndApplyInputCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &FreeIpaTestAndApplyInputCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *FreeIpaTestAndApplyInputClient) ByID(id string) (*FreeIpaTestAndApplyInput, error) {
	resp := &FreeIpaTestAndApplyInput{}
	err := c.apiClient.Ops.DoByID(FreeIpaTestAndApplyInputType, id, resp)
	return resp, err
}

func (c *FreeIpaTestAndApplyInputClient) Delete(container *FreeIpaTestAndApplyInput) error {
	return c.apiClient.Ops.DoResourceDelete(FreeIpaTestAndApplyInputType, &container.Resource)
}
