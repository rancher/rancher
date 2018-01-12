package client

import (
	"github.com/rancher/norman/types"
)

const (
	BasicAuthType                 = "basicAuth"
	BasicAuthFieldAnnotations     = "annotations"
	BasicAuthFieldCreated         = "created"
	BasicAuthFieldCreatorID       = "creatorId"
	BasicAuthFieldDescription     = "description"
	BasicAuthFieldLabels          = "labels"
	BasicAuthFieldName            = "name"
	BasicAuthFieldNamespaceId     = "namespaceId"
	BasicAuthFieldOwnerReferences = "ownerReferences"
	BasicAuthFieldPassword        = "password"
	BasicAuthFieldProjectID       = "projectId"
	BasicAuthFieldRemoved         = "removed"
	BasicAuthFieldUsername        = "username"
	BasicAuthFieldUuid            = "uuid"
)

type BasicAuth struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Description     string            `json:"description,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Password        string            `json:"password,omitempty"`
	ProjectID       string            `json:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Username        string            `json:"username,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type BasicAuthCollection struct {
	types.Collection
	Data   []BasicAuth `json:"data,omitempty"`
	client *BasicAuthClient
}

type BasicAuthClient struct {
	apiClient *Client
}

type BasicAuthOperations interface {
	List(opts *types.ListOpts) (*BasicAuthCollection, error)
	Create(opts *BasicAuth) (*BasicAuth, error)
	Update(existing *BasicAuth, updates interface{}) (*BasicAuth, error)
	ByID(id string) (*BasicAuth, error)
	Delete(container *BasicAuth) error
}

func newBasicAuthClient(apiClient *Client) *BasicAuthClient {
	return &BasicAuthClient{
		apiClient: apiClient,
	}
}

func (c *BasicAuthClient) Create(container *BasicAuth) (*BasicAuth, error) {
	resp := &BasicAuth{}
	err := c.apiClient.Ops.DoCreate(BasicAuthType, container, resp)
	return resp, err
}

func (c *BasicAuthClient) Update(existing *BasicAuth, updates interface{}) (*BasicAuth, error) {
	resp := &BasicAuth{}
	err := c.apiClient.Ops.DoUpdate(BasicAuthType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *BasicAuthClient) List(opts *types.ListOpts) (*BasicAuthCollection, error) {
	resp := &BasicAuthCollection{}
	err := c.apiClient.Ops.DoList(BasicAuthType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *BasicAuthCollection) Next() (*BasicAuthCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &BasicAuthCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *BasicAuthClient) ByID(id string) (*BasicAuth, error) {
	resp := &BasicAuth{}
	err := c.apiClient.Ops.DoByID(BasicAuthType, id, resp)
	return resp, err
}

func (c *BasicAuthClient) Delete(container *BasicAuth) error {
	return c.apiClient.Ops.DoResourceDelete(BasicAuthType, &container.Resource)
}
