package client

import (
	"github.com/rancher/norman/types"
)

const (
	LoginInputType                          = "loginInput"
	LoginInputFieldAnnotations              = "annotations"
	LoginInputFieldCreated                  = "created"
	LoginInputFieldDescription              = "description"
	LoginInputFieldFinalizers               = "finalizers"
	LoginInputFieldGithubCredential         = "githubCredential"
	LoginInputFieldIdentityRefreshTTLMillis = "identityRefreshTTL"
	LoginInputFieldLabels                   = "labels"
	LoginInputFieldLocalCredential          = "localCredential"
	LoginInputFieldName                     = "name"
	LoginInputFieldOwnerReferences          = "ownerReferences"
	LoginInputFieldRemoved                  = "removed"
	LoginInputFieldResourcePath             = "resourcePath"
	LoginInputFieldResponseType             = "responseType"
	LoginInputFieldTTLMillis                = "ttl"
	LoginInputFieldUuid                     = "uuid"
)

type LoginInput struct {
	types.Resource
	Annotations              map[string]string `json:"annotations,omitempty"`
	Created                  string            `json:"created,omitempty"`
	Description              string            `json:"description,omitempty"`
	Finalizers               []string          `json:"finalizers,omitempty"`
	GithubCredential         *GithubCredential `json:"githubCredential,omitempty"`
	IdentityRefreshTTLMillis string            `json:"identityRefreshTTL,omitempty"`
	Labels                   map[string]string `json:"labels,omitempty"`
	LocalCredential          *LocalCredential  `json:"localCredential,omitempty"`
	Name                     string            `json:"name,omitempty"`
	OwnerReferences          []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed                  string            `json:"removed,omitempty"`
	ResourcePath             string            `json:"resourcePath,omitempty"`
	ResponseType             string            `json:"responseType,omitempty"`
	TTLMillis                string            `json:"ttl,omitempty"`
	Uuid                     string            `json:"uuid,omitempty"`
}
type LoginInputCollection struct {
	types.Collection
	Data   []LoginInput `json:"data,omitempty"`
	client *LoginInputClient
}

type LoginInputClient struct {
	apiClient *Client
}

type LoginInputOperations interface {
	List(opts *types.ListOpts) (*LoginInputCollection, error)
	Create(opts *LoginInput) (*LoginInput, error)
	Update(existing *LoginInput, updates interface{}) (*LoginInput, error)
	ByID(id string) (*LoginInput, error)
	Delete(container *LoginInput) error
}

func newLoginInputClient(apiClient *Client) *LoginInputClient {
	return &LoginInputClient{
		apiClient: apiClient,
	}
}

func (c *LoginInputClient) Create(container *LoginInput) (*LoginInput, error) {
	resp := &LoginInput{}
	err := c.apiClient.Ops.DoCreate(LoginInputType, container, resp)
	return resp, err
}

func (c *LoginInputClient) Update(existing *LoginInput, updates interface{}) (*LoginInput, error) {
	resp := &LoginInput{}
	err := c.apiClient.Ops.DoUpdate(LoginInputType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *LoginInputClient) List(opts *types.ListOpts) (*LoginInputCollection, error) {
	resp := &LoginInputCollection{}
	err := c.apiClient.Ops.DoList(LoginInputType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *LoginInputCollection) Next() (*LoginInputCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &LoginInputCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *LoginInputClient) ByID(id string) (*LoginInput, error) {
	resp := &LoginInput{}
	err := c.apiClient.Ops.DoByID(LoginInputType, id, resp)
	return resp, err
}

func (c *LoginInputClient) Delete(container *LoginInput) error {
	return c.apiClient.Ops.DoResourceDelete(LoginInputType, &container.Resource)
}
