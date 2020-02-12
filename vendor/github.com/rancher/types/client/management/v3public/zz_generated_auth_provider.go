package client

import (
	"github.com/rancher/norman/types"
)

const (
	AuthProviderType                 = "authProvider"
	AuthProviderFieldAnnotations     = "annotations"
	AuthProviderFieldCreated         = "created"
	AuthProviderFieldCreatorID       = "creatorId"
	AuthProviderFieldLabels          = "labels"
	AuthProviderFieldName            = "name"
	AuthProviderFieldOwnerReferences = "ownerReferences"
	AuthProviderFieldRemoved         = "removed"
	AuthProviderFieldType            = "type"
	AuthProviderFieldUUID            = "uuid"
)

type AuthProvider struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type AuthProviderCollection struct {
	types.Collection
	Data   []AuthProvider `json:"data,omitempty"`
	client *AuthProviderClient
}

type AuthProviderClient struct {
	apiClient *Client
}

type AuthProviderOperations interface {
	List(opts *types.ListOpts) (*AuthProviderCollection, error)
	ListAll(opts *types.ListOpts) (*AuthProviderCollection, error)
	Create(opts *AuthProvider) (*AuthProvider, error)
	Update(existing *AuthProvider, updates interface{}) (*AuthProvider, error)
	Replace(existing *AuthProvider) (*AuthProvider, error)
	ByID(id string) (*AuthProvider, error)
	Delete(container *AuthProvider) error
}

func newAuthProviderClient(apiClient *Client) *AuthProviderClient {
	return &AuthProviderClient{
		apiClient: apiClient,
	}
}

func (c *AuthProviderClient) Create(container *AuthProvider) (*AuthProvider, error) {
	resp := &AuthProvider{}
	err := c.apiClient.Ops.DoCreate(AuthProviderType, container, resp)
	return resp, err
}

func (c *AuthProviderClient) Update(existing *AuthProvider, updates interface{}) (*AuthProvider, error) {
	resp := &AuthProvider{}
	err := c.apiClient.Ops.DoUpdate(AuthProviderType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *AuthProviderClient) Replace(obj *AuthProvider) (*AuthProvider, error) {
	resp := &AuthProvider{}
	err := c.apiClient.Ops.DoReplace(AuthProviderType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *AuthProviderClient) List(opts *types.ListOpts) (*AuthProviderCollection, error) {
	resp := &AuthProviderCollection{}
	err := c.apiClient.Ops.DoList(AuthProviderType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *AuthProviderClient) ListAll(opts *types.ListOpts) (*AuthProviderCollection, error) {
	resp := &AuthProviderCollection{}
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

func (cc *AuthProviderCollection) Next() (*AuthProviderCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &AuthProviderCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *AuthProviderClient) ByID(id string) (*AuthProvider, error) {
	resp := &AuthProvider{}
	err := c.apiClient.Ops.DoByID(AuthProviderType, id, resp)
	return resp, err
}

func (c *AuthProviderClient) Delete(container *AuthProvider) error {
	return c.apiClient.Ops.DoResourceDelete(AuthProviderType, &container.Resource)
}
