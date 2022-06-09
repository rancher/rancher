package client

import (
	"github.com/rancher/norman/types"
)

const (
	RkeAddonType                 = "rkeAddon"
	RkeAddonFieldAnnotations     = "annotations"
	RkeAddonFieldCreated         = "created"
	RkeAddonFieldCreatorID       = "creatorId"
	RkeAddonFieldLabels          = "labels"
	RkeAddonFieldName            = "name"
	RkeAddonFieldOwnerReferences = "ownerReferences"
	RkeAddonFieldRemoved         = "removed"
	RkeAddonFieldTemplate        = "template"
	RkeAddonFieldUUID            = "uuid"
)

type RkeAddon struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Template        string            `json:"template,omitempty" yaml:"template,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type RkeAddonCollection struct {
	types.Collection
	Data   []RkeAddon `json:"data,omitempty"`
	client *RkeAddonClient
}

type RkeAddonClient struct {
	apiClient *Client
}

type RkeAddonOperations interface {
	List(opts *types.ListOpts) (*RkeAddonCollection, error)
	ListAll(opts *types.ListOpts) (*RkeAddonCollection, error)
	Create(opts *RkeAddon) (*RkeAddon, error)
	Update(existing *RkeAddon, updates interface{}) (*RkeAddon, error)
	Replace(existing *RkeAddon) (*RkeAddon, error)
	ByID(id string) (*RkeAddon, error)
	Delete(container *RkeAddon) error
}

func newRkeAddonClient(apiClient *Client) *RkeAddonClient {
	return &RkeAddonClient{
		apiClient: apiClient,
	}
}

func (c *RkeAddonClient) Create(container *RkeAddon) (*RkeAddon, error) {
	resp := &RkeAddon{}
	err := c.apiClient.Ops.DoCreate(RkeAddonType, container, resp)
	return resp, err
}

func (c *RkeAddonClient) Update(existing *RkeAddon, updates interface{}) (*RkeAddon, error) {
	resp := &RkeAddon{}
	err := c.apiClient.Ops.DoUpdate(RkeAddonType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RkeAddonClient) Replace(obj *RkeAddon) (*RkeAddon, error) {
	resp := &RkeAddon{}
	err := c.apiClient.Ops.DoReplace(RkeAddonType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RkeAddonClient) List(opts *types.ListOpts) (*RkeAddonCollection, error) {
	resp := &RkeAddonCollection{}
	err := c.apiClient.Ops.DoList(RkeAddonType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *RkeAddonClient) ListAll(opts *types.ListOpts) (*RkeAddonCollection, error) {
	resp := &RkeAddonCollection{}
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

func (cc *RkeAddonCollection) Next() (*RkeAddonCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RkeAddonCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RkeAddonClient) ByID(id string) (*RkeAddon, error) {
	resp := &RkeAddon{}
	err := c.apiClient.Ops.DoByID(RkeAddonType, id, resp)
	return resp, err
}

func (c *RkeAddonClient) Delete(container *RkeAddon) error {
	return c.apiClient.Ops.DoResourceDelete(RkeAddonType, &container.Resource)
}
