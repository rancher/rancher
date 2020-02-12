package client

import (
	"github.com/rancher/norman/types"
)

const (
	RKEAddonType                 = "rkeAddon"
	RKEAddonFieldAnnotations     = "annotations"
	RKEAddonFieldCreated         = "created"
	RKEAddonFieldCreatorID       = "creatorId"
	RKEAddonFieldLabels          = "labels"
	RKEAddonFieldName            = "name"
	RKEAddonFieldOwnerReferences = "ownerReferences"
	RKEAddonFieldRemoved         = "removed"
	RKEAddonFieldTemplate        = "template"
	RKEAddonFieldUUID            = "uuid"
)

type RKEAddon struct {
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

type RKEAddonCollection struct {
	types.Collection
	Data   []RKEAddon `json:"data,omitempty"`
	client *RKEAddonClient
}

type RKEAddonClient struct {
	apiClient *Client
}

type RKEAddonOperations interface {
	List(opts *types.ListOpts) (*RKEAddonCollection, error)
	ListAll(opts *types.ListOpts) (*RKEAddonCollection, error)
	Create(opts *RKEAddon) (*RKEAddon, error)
	Update(existing *RKEAddon, updates interface{}) (*RKEAddon, error)
	Replace(existing *RKEAddon) (*RKEAddon, error)
	ByID(id string) (*RKEAddon, error)
	Delete(container *RKEAddon) error
}

func newRKEAddonClient(apiClient *Client) *RKEAddonClient {
	return &RKEAddonClient{
		apiClient: apiClient,
	}
}

func (c *RKEAddonClient) Create(container *RKEAddon) (*RKEAddon, error) {
	resp := &RKEAddon{}
	err := c.apiClient.Ops.DoCreate(RKEAddonType, container, resp)
	return resp, err
}

func (c *RKEAddonClient) Update(existing *RKEAddon, updates interface{}) (*RKEAddon, error) {
	resp := &RKEAddon{}
	err := c.apiClient.Ops.DoUpdate(RKEAddonType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RKEAddonClient) Replace(obj *RKEAddon) (*RKEAddon, error) {
	resp := &RKEAddon{}
	err := c.apiClient.Ops.DoReplace(RKEAddonType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RKEAddonClient) List(opts *types.ListOpts) (*RKEAddonCollection, error) {
	resp := &RKEAddonCollection{}
	err := c.apiClient.Ops.DoList(RKEAddonType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *RKEAddonClient) ListAll(opts *types.ListOpts) (*RKEAddonCollection, error) {
	resp := &RKEAddonCollection{}
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

func (cc *RKEAddonCollection) Next() (*RKEAddonCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RKEAddonCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RKEAddonClient) ByID(id string) (*RKEAddon, error) {
	resp := &RKEAddon{}
	err := c.apiClient.Ops.DoByID(RKEAddonType, id, resp)
	return resp, err
}

func (c *RKEAddonClient) Delete(container *RKEAddon) error {
	return c.apiClient.Ops.DoResourceDelete(RKEAddonType, &container.Resource)
}
