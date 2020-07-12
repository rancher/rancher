package client

import (
	"github.com/rancher/norman/types"
)

const (
	RkeK8sSystemImageType                 = "rkeK8sSystemImage"
	RkeK8sSystemImageFieldAnnotations     = "annotations"
	RkeK8sSystemImageFieldCreated         = "created"
	RkeK8sSystemImageFieldCreatorID       = "creatorId"
	RkeK8sSystemImageFieldLabels          = "labels"
	RkeK8sSystemImageFieldName            = "name"
	RkeK8sSystemImageFieldOwnerReferences = "ownerReferences"
	RkeK8sSystemImageFieldRemoved         = "removed"
	RkeK8sSystemImageFieldSystemImages    = "systemImages"
	RkeK8sSystemImageFieldUUID            = "uuid"
)

type RkeK8sSystemImage struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SystemImages    *RKESystemImages  `json:"systemImages,omitempty" yaml:"systemImages,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type RkeK8sSystemImageCollection struct {
	types.Collection
	Data   []RkeK8sSystemImage `json:"data,omitempty"`
	client *RkeK8sSystemImageClient
}

type RkeK8sSystemImageClient struct {
	apiClient *Client
}

type RkeK8sSystemImageOperations interface {
	List(opts *types.ListOpts) (*RkeK8sSystemImageCollection, error)
	ListAll(opts *types.ListOpts) (*RkeK8sSystemImageCollection, error)
	Create(opts *RkeK8sSystemImage) (*RkeK8sSystemImage, error)
	Update(existing *RkeK8sSystemImage, updates interface{}) (*RkeK8sSystemImage, error)
	Replace(existing *RkeK8sSystemImage) (*RkeK8sSystemImage, error)
	ByID(id string) (*RkeK8sSystemImage, error)
	Delete(container *RkeK8sSystemImage) error
}

func newRkeK8sSystemImageClient(apiClient *Client) *RkeK8sSystemImageClient {
	return &RkeK8sSystemImageClient{
		apiClient: apiClient,
	}
}

func (c *RkeK8sSystemImageClient) Create(container *RkeK8sSystemImage) (*RkeK8sSystemImage, error) {
	resp := &RkeK8sSystemImage{}
	err := c.apiClient.Ops.DoCreate(RkeK8sSystemImageType, container, resp)
	return resp, err
}

func (c *RkeK8sSystemImageClient) Update(existing *RkeK8sSystemImage, updates interface{}) (*RkeK8sSystemImage, error) {
	resp := &RkeK8sSystemImage{}
	err := c.apiClient.Ops.DoUpdate(RkeK8sSystemImageType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RkeK8sSystemImageClient) Replace(obj *RkeK8sSystemImage) (*RkeK8sSystemImage, error) {
	resp := &RkeK8sSystemImage{}
	err := c.apiClient.Ops.DoReplace(RkeK8sSystemImageType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RkeK8sSystemImageClient) List(opts *types.ListOpts) (*RkeK8sSystemImageCollection, error) {
	resp := &RkeK8sSystemImageCollection{}
	err := c.apiClient.Ops.DoList(RkeK8sSystemImageType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *RkeK8sSystemImageClient) ListAll(opts *types.ListOpts) (*RkeK8sSystemImageCollection, error) {
	resp := &RkeK8sSystemImageCollection{}
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

func (cc *RkeK8sSystemImageCollection) Next() (*RkeK8sSystemImageCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RkeK8sSystemImageCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RkeK8sSystemImageClient) ByID(id string) (*RkeK8sSystemImage, error) {
	resp := &RkeK8sSystemImage{}
	err := c.apiClient.Ops.DoByID(RkeK8sSystemImageType, id, resp)
	return resp, err
}

func (c *RkeK8sSystemImageClient) Delete(container *RkeK8sSystemImage) error {
	return c.apiClient.Ops.DoResourceDelete(RkeK8sSystemImageType, &container.Resource)
}
