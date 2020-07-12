package client

import (
	"github.com/rancher/norman/types"
)

const (
	NamespacedBasicAuthType                 = "namespacedBasicAuth"
	NamespacedBasicAuthFieldAnnotations     = "annotations"
	NamespacedBasicAuthFieldCreated         = "created"
	NamespacedBasicAuthFieldCreatorID       = "creatorId"
	NamespacedBasicAuthFieldDescription     = "description"
	NamespacedBasicAuthFieldLabels          = "labels"
	NamespacedBasicAuthFieldName            = "name"
	NamespacedBasicAuthFieldNamespaceId     = "namespaceId"
	NamespacedBasicAuthFieldOwnerReferences = "ownerReferences"
	NamespacedBasicAuthFieldPassword        = "password"
	NamespacedBasicAuthFieldProjectID       = "projectId"
	NamespacedBasicAuthFieldRemoved         = "removed"
	NamespacedBasicAuthFieldUUID            = "uuid"
	NamespacedBasicAuthFieldUsername        = "username"
)

type NamespacedBasicAuth struct {
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

type NamespacedBasicAuthCollection struct {
	types.Collection
	Data   []NamespacedBasicAuth `json:"data,omitempty"`
	client *NamespacedBasicAuthClient
}

type NamespacedBasicAuthClient struct {
	apiClient *Client
}

type NamespacedBasicAuthOperations interface {
	List(opts *types.ListOpts) (*NamespacedBasicAuthCollection, error)
	ListAll(opts *types.ListOpts) (*NamespacedBasicAuthCollection, error)
	Create(opts *NamespacedBasicAuth) (*NamespacedBasicAuth, error)
	Update(existing *NamespacedBasicAuth, updates interface{}) (*NamespacedBasicAuth, error)
	Replace(existing *NamespacedBasicAuth) (*NamespacedBasicAuth, error)
	ByID(id string) (*NamespacedBasicAuth, error)
	Delete(container *NamespacedBasicAuth) error
}

func newNamespacedBasicAuthClient(apiClient *Client) *NamespacedBasicAuthClient {
	return &NamespacedBasicAuthClient{
		apiClient: apiClient,
	}
}

func (c *NamespacedBasicAuthClient) Create(container *NamespacedBasicAuth) (*NamespacedBasicAuth, error) {
	resp := &NamespacedBasicAuth{}
	err := c.apiClient.Ops.DoCreate(NamespacedBasicAuthType, container, resp)
	return resp, err
}

func (c *NamespacedBasicAuthClient) Update(existing *NamespacedBasicAuth, updates interface{}) (*NamespacedBasicAuth, error) {
	resp := &NamespacedBasicAuth{}
	err := c.apiClient.Ops.DoUpdate(NamespacedBasicAuthType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NamespacedBasicAuthClient) Replace(obj *NamespacedBasicAuth) (*NamespacedBasicAuth, error) {
	resp := &NamespacedBasicAuth{}
	err := c.apiClient.Ops.DoReplace(NamespacedBasicAuthType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *NamespacedBasicAuthClient) List(opts *types.ListOpts) (*NamespacedBasicAuthCollection, error) {
	resp := &NamespacedBasicAuthCollection{}
	err := c.apiClient.Ops.DoList(NamespacedBasicAuthType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *NamespacedBasicAuthClient) ListAll(opts *types.ListOpts) (*NamespacedBasicAuthCollection, error) {
	resp := &NamespacedBasicAuthCollection{}
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

func (cc *NamespacedBasicAuthCollection) Next() (*NamespacedBasicAuthCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NamespacedBasicAuthCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NamespacedBasicAuthClient) ByID(id string) (*NamespacedBasicAuth, error) {
	resp := &NamespacedBasicAuth{}
	err := c.apiClient.Ops.DoByID(NamespacedBasicAuthType, id, resp)
	return resp, err
}

func (c *NamespacedBasicAuthClient) Delete(container *NamespacedBasicAuth) error {
	return c.apiClient.Ops.DoResourceDelete(NamespacedBasicAuthType, &container.Resource)
}
