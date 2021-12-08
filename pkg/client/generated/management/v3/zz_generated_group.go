package client

import (
	"github.com/rancher/norman/types"
)

const (
	GroupType                 = "group"
	GroupFieldAnnotations     = "annotations"
	GroupFieldCreated         = "created"
	GroupFieldCreatorID       = "creatorId"
	GroupFieldLabels          = "labels"
	GroupFieldName            = "name"
	GroupFieldOwnerReferences = "ownerReferences"
	GroupFieldRemoved         = "removed"
	GroupFieldUUID            = "uuid"
)

type Group struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type GroupCollection struct {
	types.Collection
	Data   []Group `json:"data,omitempty"`
	client *GroupClient
}

type GroupClient struct {
	apiClient *Client
}

type GroupOperations interface {
	List(opts *types.ListOpts) (*GroupCollection, error)
	ListAll(opts *types.ListOpts) (*GroupCollection, error)
	Create(opts *Group) (*Group, error)
	Update(existing *Group, updates interface{}) (*Group, error)
	Replace(existing *Group) (*Group, error)
	ByID(id string) (*Group, error)
	Delete(container *Group) error
}

func newGroupClient(apiClient *Client) *GroupClient {
	return &GroupClient{
		apiClient: apiClient,
	}
}

func (c *GroupClient) Create(container *Group) (*Group, error) {
	resp := &Group{}
	err := c.apiClient.Ops.DoCreate(GroupType, container, resp)
	return resp, err
}

func (c *GroupClient) Update(existing *Group, updates interface{}) (*Group, error) {
	resp := &Group{}
	err := c.apiClient.Ops.DoUpdate(GroupType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GroupClient) Replace(obj *Group) (*Group, error) {
	resp := &Group{}
	err := c.apiClient.Ops.DoReplace(GroupType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *GroupClient) List(opts *types.ListOpts) (*GroupCollection, error) {
	resp := &GroupCollection{}
	err := c.apiClient.Ops.DoList(GroupType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *GroupClient) ListAll(opts *types.ListOpts) (*GroupCollection, error) {
	resp := &GroupCollection{}
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

func (cc *GroupCollection) Next() (*GroupCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GroupCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GroupClient) ByID(id string) (*Group, error) {
	resp := &Group{}
	err := c.apiClient.Ops.DoByID(GroupType, id, resp)
	return resp, err
}

func (c *GroupClient) Delete(container *Group) error {
	return c.apiClient.Ops.DoResourceDelete(GroupType, &container.Resource)
}
