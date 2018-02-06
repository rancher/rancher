package client

import (
	"github.com/rancher/norman/types"
)

const (
	AuthConfigType                 = "authConfig"
	AuthConfigFieldAnnotations     = "annotations"
	AuthConfigFieldCreated         = "created"
	AuthConfigFieldCreatorID       = "creatorId"
	AuthConfigFieldEnabled         = "enabled"
	AuthConfigFieldLabels          = "labels"
	AuthConfigFieldName            = "name"
	AuthConfigFieldOwnerReferences = "ownerReferences"
	AuthConfigFieldRemoved         = "removed"
	AuthConfigFieldType            = "type"
	AuthConfigFieldUuid            = "uuid"
)

type AuthConfig struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Enabled         *bool             `json:"enabled,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Type            string            `json:"type,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type AuthConfigCollection struct {
	types.Collection
	Data   []AuthConfig `json:"data,omitempty"`
	client *AuthConfigClient
}

type AuthConfigClient struct {
	apiClient *Client
}

type AuthConfigOperations interface {
	List(opts *types.ListOpts) (*AuthConfigCollection, error)
	Create(opts *AuthConfig) (*AuthConfig, error)
	Update(existing *AuthConfig, updates interface{}) (*AuthConfig, error)
	ByID(id string) (*AuthConfig, error)
	Delete(container *AuthConfig) error
}

func newAuthConfigClient(apiClient *Client) *AuthConfigClient {
	return &AuthConfigClient{
		apiClient: apiClient,
	}
}

func (c *AuthConfigClient) Create(container *AuthConfig) (*AuthConfig, error) {
	resp := &AuthConfig{}
	err := c.apiClient.Ops.DoCreate(AuthConfigType, container, resp)
	return resp, err
}

func (c *AuthConfigClient) Update(existing *AuthConfig, updates interface{}) (*AuthConfig, error) {
	resp := &AuthConfig{}
	err := c.apiClient.Ops.DoUpdate(AuthConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *AuthConfigClient) List(opts *types.ListOpts) (*AuthConfigCollection, error) {
	resp := &AuthConfigCollection{}
	err := c.apiClient.Ops.DoList(AuthConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *AuthConfigCollection) Next() (*AuthConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &AuthConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *AuthConfigClient) ByID(id string) (*AuthConfig, error) {
	resp := &AuthConfig{}
	err := c.apiClient.Ops.DoByID(AuthConfigType, id, resp)
	return resp, err
}

func (c *AuthConfigClient) Delete(container *AuthConfig) error {
	return c.apiClient.Ops.DoResourceDelete(AuthConfigType, &container.Resource)
}
