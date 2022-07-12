package client

import (
	"github.com/rancher/norman/types"
)

const (
	RKEControlPlaneType            = "rkeControlPlane"
	RKEControlPlaneFieldCreatorID  = "creatorId"
	RKEControlPlaneFieldObjectMeta = "metadata"
	RKEControlPlaneFieldSpec       = "spec"
	RKEControlPlaneFieldStatus     = "status"
)

type RKEControlPlane struct {
	types.Resource
	CreatorID  string                 `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ObjectMeta *ObjectMeta            `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       *RKEControlPlaneSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status     *RKEControlPlaneStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type RKEControlPlaneCollection struct {
	types.Collection
	Data   []RKEControlPlane `json:"data,omitempty"`
	client *RKEControlPlaneClient
}

type RKEControlPlaneClient struct {
	apiClient *Client
}

type RKEControlPlaneOperations interface {
	List(opts *types.ListOpts) (*RKEControlPlaneCollection, error)
	ListAll(opts *types.ListOpts) (*RKEControlPlaneCollection, error)
	Create(opts *RKEControlPlane) (*RKEControlPlane, error)
	Update(existing *RKEControlPlane, updates interface{}) (*RKEControlPlane, error)
	Replace(existing *RKEControlPlane) (*RKEControlPlane, error)
	ByID(id string) (*RKEControlPlane, error)
	Delete(container *RKEControlPlane) error
}

func newRKEControlPlaneClient(apiClient *Client) *RKEControlPlaneClient {
	return &RKEControlPlaneClient{
		apiClient: apiClient,
	}
}

func (c *RKEControlPlaneClient) Create(container *RKEControlPlane) (*RKEControlPlane, error) {
	resp := &RKEControlPlane{}
	err := c.apiClient.Ops.DoCreate(RKEControlPlaneType, container, resp)
	return resp, err
}

func (c *RKEControlPlaneClient) Update(existing *RKEControlPlane, updates interface{}) (*RKEControlPlane, error) {
	resp := &RKEControlPlane{}
	err := c.apiClient.Ops.DoUpdate(RKEControlPlaneType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RKEControlPlaneClient) Replace(obj *RKEControlPlane) (*RKEControlPlane, error) {
	resp := &RKEControlPlane{}
	err := c.apiClient.Ops.DoReplace(RKEControlPlaneType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RKEControlPlaneClient) List(opts *types.ListOpts) (*RKEControlPlaneCollection, error) {
	resp := &RKEControlPlaneCollection{}
	err := c.apiClient.Ops.DoList(RKEControlPlaneType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *RKEControlPlaneClient) ListAll(opts *types.ListOpts) (*RKEControlPlaneCollection, error) {
	resp := &RKEControlPlaneCollection{}
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

func (cc *RKEControlPlaneCollection) Next() (*RKEControlPlaneCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RKEControlPlaneCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RKEControlPlaneClient) ByID(id string) (*RKEControlPlane, error) {
	resp := &RKEControlPlane{}
	err := c.apiClient.Ops.DoByID(RKEControlPlaneType, id, resp)
	return resp, err
}

func (c *RKEControlPlaneClient) Delete(container *RKEControlPlane) error {
	return c.apiClient.Ops.DoResourceDelete(RKEControlPlaneType, &container.Resource)
}
