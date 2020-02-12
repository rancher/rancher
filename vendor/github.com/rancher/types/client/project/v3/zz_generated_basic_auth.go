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
	BasicAuthFieldUUID            = "uuid"
	BasicAuthFieldUsername        = "username"
)

type BasicAuth struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Password        string            `json:"password,omitempty" yaml:"password,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Username        string            `json:"username,omitempty" yaml:"username,omitempty"`
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
	ListAll(opts *types.ListOpts) (*BasicAuthCollection, error)
	Create(opts *BasicAuth) (*BasicAuth, error)
	Update(existing *BasicAuth, updates interface{}) (*BasicAuth, error)
	Replace(existing *BasicAuth) (*BasicAuth, error)
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

func (c *BasicAuthClient) Replace(obj *BasicAuth) (*BasicAuth, error) {
	resp := &BasicAuth{}
	err := c.apiClient.Ops.DoReplace(BasicAuthType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *BasicAuthClient) List(opts *types.ListOpts) (*BasicAuthCollection, error) {
	resp := &BasicAuthCollection{}
	err := c.apiClient.Ops.DoList(BasicAuthType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *BasicAuthClient) ListAll(opts *types.ListOpts) (*BasicAuthCollection, error) {
	resp := &BasicAuthCollection{}
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
