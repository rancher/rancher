package client

import (
	"github.com/rancher/norman/types"
)

const (
	SourceCodeProviderConfigType                 = "sourceCodeProviderConfig"
	SourceCodeProviderConfigFieldAnnotations     = "annotations"
	SourceCodeProviderConfigFieldCreated         = "created"
	SourceCodeProviderConfigFieldCreatorID       = "creatorId"
	SourceCodeProviderConfigFieldEnabled         = "enabled"
	SourceCodeProviderConfigFieldLabels          = "labels"
	SourceCodeProviderConfigFieldName            = "name"
	SourceCodeProviderConfigFieldNamespaceId     = "namespaceId"
	SourceCodeProviderConfigFieldOwnerReferences = "ownerReferences"
	SourceCodeProviderConfigFieldProjectID       = "projectId"
	SourceCodeProviderConfigFieldRemoved         = "removed"
	SourceCodeProviderConfigFieldType            = "type"
	SourceCodeProviderConfigFieldUUID            = "uuid"
)

type SourceCodeProviderConfig struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled         bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type SourceCodeProviderConfigCollection struct {
	types.Collection
	Data   []SourceCodeProviderConfig `json:"data,omitempty"`
	client *SourceCodeProviderConfigClient
}

type SourceCodeProviderConfigClient struct {
	apiClient *Client
}

type SourceCodeProviderConfigOperations interface {
	List(opts *types.ListOpts) (*SourceCodeProviderConfigCollection, error)
	ListAll(opts *types.ListOpts) (*SourceCodeProviderConfigCollection, error)
	Create(opts *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error)
	Update(existing *SourceCodeProviderConfig, updates interface{}) (*SourceCodeProviderConfig, error)
	Replace(existing *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error)
	ByID(id string) (*SourceCodeProviderConfig, error)
	Delete(container *SourceCodeProviderConfig) error
}

func newSourceCodeProviderConfigClient(apiClient *Client) *SourceCodeProviderConfigClient {
	return &SourceCodeProviderConfigClient{
		apiClient: apiClient,
	}
}

func (c *SourceCodeProviderConfigClient) Create(container *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error) {
	resp := &SourceCodeProviderConfig{}
	err := c.apiClient.Ops.DoCreate(SourceCodeProviderConfigType, container, resp)
	return resp, err
}

func (c *SourceCodeProviderConfigClient) Update(existing *SourceCodeProviderConfig, updates interface{}) (*SourceCodeProviderConfig, error) {
	resp := &SourceCodeProviderConfig{}
	err := c.apiClient.Ops.DoUpdate(SourceCodeProviderConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *SourceCodeProviderConfigClient) Replace(obj *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error) {
	resp := &SourceCodeProviderConfig{}
	err := c.apiClient.Ops.DoReplace(SourceCodeProviderConfigType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *SourceCodeProviderConfigClient) List(opts *types.ListOpts) (*SourceCodeProviderConfigCollection, error) {
	resp := &SourceCodeProviderConfigCollection{}
	err := c.apiClient.Ops.DoList(SourceCodeProviderConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *SourceCodeProviderConfigClient) ListAll(opts *types.ListOpts) (*SourceCodeProviderConfigCollection, error) {
	resp := &SourceCodeProviderConfigCollection{}
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

func (cc *SourceCodeProviderConfigCollection) Next() (*SourceCodeProviderConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &SourceCodeProviderConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *SourceCodeProviderConfigClient) ByID(id string) (*SourceCodeProviderConfig, error) {
	resp := &SourceCodeProviderConfig{}
	err := c.apiClient.Ops.DoByID(SourceCodeProviderConfigType, id, resp)
	return resp, err
}

func (c *SourceCodeProviderConfigClient) Delete(container *SourceCodeProviderConfig) error {
	return c.apiClient.Ops.DoResourceDelete(SourceCodeProviderConfigType, &container.Resource)
}
