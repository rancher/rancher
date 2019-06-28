package client

import (
	"github.com/rancher/norman/types"
)

const (
	RKEK8sSystemImageType                 = "rkeK8sSystemImage"
	RKEK8sSystemImageFieldAnnotations     = "annotations"
	RKEK8sSystemImageFieldCreated         = "created"
	RKEK8sSystemImageFieldCreatorID       = "creatorId"
	RKEK8sSystemImageFieldLabels          = "labels"
	RKEK8sSystemImageFieldName            = "name"
	RKEK8sSystemImageFieldOwnerReferences = "ownerReferences"
	RKEK8sSystemImageFieldRemoved         = "removed"
	RKEK8sSystemImageFieldSystemImages    = "systemImages"
	RKEK8sSystemImageFieldUUID            = "uuid"
)

type RKEK8sSystemImage struct {
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

type RKEK8sSystemImageCollection struct {
	types.Collection
	Data   []RKEK8sSystemImage `json:"data,omitempty"`
	client *RKEK8sSystemImageClient
}

type RKEK8sSystemImageClient struct {
	apiClient *Client
}

type RKEK8sSystemImageOperations interface {
	List(opts *types.ListOpts) (*RKEK8sSystemImageCollection, error)
	Create(opts *RKEK8sSystemImage) (*RKEK8sSystemImage, error)
	Update(existing *RKEK8sSystemImage, updates interface{}) (*RKEK8sSystemImage, error)
	Replace(existing *RKEK8sSystemImage) (*RKEK8sSystemImage, error)
	ByID(id string) (*RKEK8sSystemImage, error)
	Delete(container *RKEK8sSystemImage) error
}

func newRKEK8sSystemImageClient(apiClient *Client) *RKEK8sSystemImageClient {
	return &RKEK8sSystemImageClient{
		apiClient: apiClient,
	}
}

func (c *RKEK8sSystemImageClient) Create(container *RKEK8sSystemImage) (*RKEK8sSystemImage, error) {
	resp := &RKEK8sSystemImage{}
	err := c.apiClient.Ops.DoCreate(RKEK8sSystemImageType, container, resp)
	return resp, err
}

func (c *RKEK8sSystemImageClient) Update(existing *RKEK8sSystemImage, updates interface{}) (*RKEK8sSystemImage, error) {
	resp := &RKEK8sSystemImage{}
	err := c.apiClient.Ops.DoUpdate(RKEK8sSystemImageType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RKEK8sSystemImageClient) Replace(obj *RKEK8sSystemImage) (*RKEK8sSystemImage, error) {
	resp := &RKEK8sSystemImage{}
	err := c.apiClient.Ops.DoReplace(RKEK8sSystemImageType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RKEK8sSystemImageClient) List(opts *types.ListOpts) (*RKEK8sSystemImageCollection, error) {
	resp := &RKEK8sSystemImageCollection{}
	err := c.apiClient.Ops.DoList(RKEK8sSystemImageType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *RKEK8sSystemImageCollection) Next() (*RKEK8sSystemImageCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RKEK8sSystemImageCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RKEK8sSystemImageClient) ByID(id string) (*RKEK8sSystemImage, error) {
	resp := &RKEK8sSystemImage{}
	err := c.apiClient.Ops.DoByID(RKEK8sSystemImageType, id, resp)
	return resp, err
}

func (c *RKEK8sSystemImageClient) Delete(container *RKEK8sSystemImage) error {
	return c.apiClient.Ops.DoResourceDelete(RKEK8sSystemImageType, &container.Resource)
}
