package client

import (
	"github.com/rancher/norman/types"
)

const (
	LocalConfigType                 = "localConfig"
	LocalConfigFieldAnnotations     = "annotations"
	LocalConfigFieldCreated         = "created"
	LocalConfigFieldCreatorID       = "creatorId"
	LocalConfigFieldLabels          = "labels"
	LocalConfigFieldName            = "name"
	LocalConfigFieldOwnerReferences = "ownerReferences"
	LocalConfigFieldRemoved         = "removed"
	LocalConfigFieldUuid            = "uuid"
)

type LocalConfig struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type LocalConfigCollection struct {
	types.Collection
	Data   []LocalConfig `json:"data,omitempty"`
	client *LocalConfigClient
}

type LocalConfigClient struct {
	apiClient *Client
}

type LocalConfigOperations interface {
	List(opts *types.ListOpts) (*LocalConfigCollection, error)
	Create(opts *LocalConfig) (*LocalConfig, error)
	Update(existing *LocalConfig, updates interface{}) (*LocalConfig, error)
	ByID(id string) (*LocalConfig, error)
	Delete(container *LocalConfig) error
}

func newLocalConfigClient(apiClient *Client) *LocalConfigClient {
	return &LocalConfigClient{
		apiClient: apiClient,
	}
}

func (c *LocalConfigClient) Create(container *LocalConfig) (*LocalConfig, error) {
	resp := &LocalConfig{}
	err := c.apiClient.Ops.DoCreate(LocalConfigType, container, resp)
	return resp, err
}

func (c *LocalConfigClient) Update(existing *LocalConfig, updates interface{}) (*LocalConfig, error) {
	resp := &LocalConfig{}
	err := c.apiClient.Ops.DoUpdate(LocalConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *LocalConfigClient) List(opts *types.ListOpts) (*LocalConfigCollection, error) {
	resp := &LocalConfigCollection{}
	err := c.apiClient.Ops.DoList(LocalConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *LocalConfigCollection) Next() (*LocalConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &LocalConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *LocalConfigClient) ByID(id string) (*LocalConfig, error) {
	resp := &LocalConfig{}
	err := c.apiClient.Ops.DoByID(LocalConfigType, id, resp)
	return resp, err
}

func (c *LocalConfigClient) Delete(container *LocalConfig) error {
	return c.apiClient.Ops.DoResourceDelete(LocalConfigType, &container.Resource)
}
