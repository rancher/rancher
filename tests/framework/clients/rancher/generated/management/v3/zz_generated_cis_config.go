package client

import (
	"github.com/rancher/norman/types"
)

const (
	CisConfigType                 = "cisConfig"
	CisConfigFieldAnnotations     = "annotations"
	CisConfigFieldCreated         = "created"
	CisConfigFieldCreatorID       = "creatorId"
	CisConfigFieldLabels          = "labels"
	CisConfigFieldName            = "name"
	CisConfigFieldOwnerReferences = "ownerReferences"
	CisConfigFieldParams          = "params"
	CisConfigFieldRemoved         = "removed"
	CisConfigFieldUUID            = "uuid"
)

type CisConfig struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Params          *CisConfigParams  `json:"params,omitempty" yaml:"params,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type CisConfigCollection struct {
	types.Collection
	Data   []CisConfig `json:"data,omitempty"`
	client *CisConfigClient
}

type CisConfigClient struct {
	apiClient *Client
}

type CisConfigOperations interface {
	List(opts *types.ListOpts) (*CisConfigCollection, error)
	ListAll(opts *types.ListOpts) (*CisConfigCollection, error)
	Create(opts *CisConfig) (*CisConfig, error)
	Update(existing *CisConfig, updates interface{}) (*CisConfig, error)
	Replace(existing *CisConfig) (*CisConfig, error)
	ByID(id string) (*CisConfig, error)
	Delete(container *CisConfig) error
}

func newCisConfigClient(apiClient *Client) *CisConfigClient {
	return &CisConfigClient{
		apiClient: apiClient,
	}
}

func (c *CisConfigClient) Create(container *CisConfig) (*CisConfig, error) {
	resp := &CisConfig{}
	err := c.apiClient.Ops.DoCreate(CisConfigType, container, resp)
	return resp, err
}

func (c *CisConfigClient) Update(existing *CisConfig, updates interface{}) (*CisConfig, error) {
	resp := &CisConfig{}
	err := c.apiClient.Ops.DoUpdate(CisConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CisConfigClient) Replace(obj *CisConfig) (*CisConfig, error) {
	resp := &CisConfig{}
	err := c.apiClient.Ops.DoReplace(CisConfigType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *CisConfigClient) List(opts *types.ListOpts) (*CisConfigCollection, error) {
	resp := &CisConfigCollection{}
	err := c.apiClient.Ops.DoList(CisConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *CisConfigClient) ListAll(opts *types.ListOpts) (*CisConfigCollection, error) {
	resp := &CisConfigCollection{}
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

func (cc *CisConfigCollection) Next() (*CisConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CisConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CisConfigClient) ByID(id string) (*CisConfig, error) {
	resp := &CisConfig{}
	err := c.apiClient.Ops.DoByID(CisConfigType, id, resp)
	return resp, err
}

func (c *CisConfigClient) Delete(container *CisConfig) error {
	return c.apiClient.Ops.DoResourceDelete(CisConfigType, &container.Resource)
}
