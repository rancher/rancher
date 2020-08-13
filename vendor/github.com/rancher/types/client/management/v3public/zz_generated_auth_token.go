package client

import (
	"github.com/rancher/norman/types"
)

const (
	AuthTokenType                 = "authToken"
	AuthTokenFieldAnnotations     = "annotations"
	AuthTokenFieldCreated         = "created"
	AuthTokenFieldCreatorID       = "creatorId"
	AuthTokenFieldExpiresAt       = "expiresAt"
	AuthTokenFieldLabels          = "labels"
	AuthTokenFieldName            = "name"
	AuthTokenFieldOwnerReferences = "ownerReferences"
	AuthTokenFieldRemoved         = "removed"
	AuthTokenFieldToken           = "token"
	AuthTokenFieldUUID            = "uuid"
)

type AuthToken struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ExpiresAt       string            `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Token           string            `json:"token,omitempty" yaml:"token,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type AuthTokenCollection struct {
	types.Collection
	Data   []AuthToken `json:"data,omitempty"`
	client *AuthTokenClient
}

type AuthTokenClient struct {
	apiClient *Client
}

type AuthTokenOperations interface {
	List(opts *types.ListOpts) (*AuthTokenCollection, error)
	ListAll(opts *types.ListOpts) (*AuthTokenCollection, error)
	Create(opts *AuthToken) (*AuthToken, error)
	Update(existing *AuthToken, updates interface{}) (*AuthToken, error)
	Replace(existing *AuthToken) (*AuthToken, error)
	ByID(id string) (*AuthToken, error)
	Delete(container *AuthToken) error
}

func newAuthTokenClient(apiClient *Client) *AuthTokenClient {
	return &AuthTokenClient{
		apiClient: apiClient,
	}
}

func (c *AuthTokenClient) Create(container *AuthToken) (*AuthToken, error) {
	resp := &AuthToken{}
	err := c.apiClient.Ops.DoCreate(AuthTokenType, container, resp)
	return resp, err
}

func (c *AuthTokenClient) Update(existing *AuthToken, updates interface{}) (*AuthToken, error) {
	resp := &AuthToken{}
	err := c.apiClient.Ops.DoUpdate(AuthTokenType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *AuthTokenClient) Replace(obj *AuthToken) (*AuthToken, error) {
	resp := &AuthToken{}
	err := c.apiClient.Ops.DoReplace(AuthTokenType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *AuthTokenClient) List(opts *types.ListOpts) (*AuthTokenCollection, error) {
	resp := &AuthTokenCollection{}
	err := c.apiClient.Ops.DoList(AuthTokenType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *AuthTokenClient) ListAll(opts *types.ListOpts) (*AuthTokenCollection, error) {
	resp := &AuthTokenCollection{}
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

func (cc *AuthTokenCollection) Next() (*AuthTokenCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &AuthTokenCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *AuthTokenClient) ByID(id string) (*AuthToken, error) {
	resp := &AuthToken{}
	err := c.apiClient.Ops.DoByID(AuthTokenType, id, resp)
	return resp, err
}

func (c *AuthTokenClient) Delete(container *AuthToken) error {
	return c.apiClient.Ops.DoResourceDelete(AuthTokenType, &container.Resource)
}
