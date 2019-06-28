package client

import (
	"github.com/rancher/norman/types"
)

const (
	RKEK8sWindowsSystemImageType                 = "rkeK8sWindowsSystemImage"
	RKEK8sWindowsSystemImageFieldAnnotations     = "annotations"
	RKEK8sWindowsSystemImageFieldCreated         = "created"
	RKEK8sWindowsSystemImageFieldCreatorID       = "creatorId"
	RKEK8sWindowsSystemImageFieldLabels          = "labels"
	RKEK8sWindowsSystemImageFieldName            = "name"
	RKEK8sWindowsSystemImageFieldOwnerReferences = "ownerReferences"
	RKEK8sWindowsSystemImageFieldRemoved         = "removed"
	RKEK8sWindowsSystemImageFieldSystemImages    = "windowsSystemImages"
	RKEK8sWindowsSystemImageFieldUUID            = "uuid"
)

type RKEK8sWindowsSystemImage struct {
	types.Resource
	Annotations     map[string]string    `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string               `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string               `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string    `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string               `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference     `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string               `json:"removed,omitempty" yaml:"removed,omitempty"`
	SystemImages    *WindowsSystemImages `json:"windowsSystemImages,omitempty" yaml:"windowsSystemImages,omitempty"`
	UUID            string               `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type RKEK8sWindowsSystemImageCollection struct {
	types.Collection
	Data   []RKEK8sWindowsSystemImage `json:"data,omitempty"`
	client *RKEK8sWindowsSystemImageClient
}

type RKEK8sWindowsSystemImageClient struct {
	apiClient *Client
}

type RKEK8sWindowsSystemImageOperations interface {
	List(opts *types.ListOpts) (*RKEK8sWindowsSystemImageCollection, error)
	Create(opts *RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error)
	Update(existing *RKEK8sWindowsSystemImage, updates interface{}) (*RKEK8sWindowsSystemImage, error)
	Replace(existing *RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error)
	ByID(id string) (*RKEK8sWindowsSystemImage, error)
	Delete(container *RKEK8sWindowsSystemImage) error
}

func newRKEK8sWindowsSystemImageClient(apiClient *Client) *RKEK8sWindowsSystemImageClient {
	return &RKEK8sWindowsSystemImageClient{
		apiClient: apiClient,
	}
}

func (c *RKEK8sWindowsSystemImageClient) Create(container *RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error) {
	resp := &RKEK8sWindowsSystemImage{}
	err := c.apiClient.Ops.DoCreate(RKEK8sWindowsSystemImageType, container, resp)
	return resp, err
}

func (c *RKEK8sWindowsSystemImageClient) Update(existing *RKEK8sWindowsSystemImage, updates interface{}) (*RKEK8sWindowsSystemImage, error) {
	resp := &RKEK8sWindowsSystemImage{}
	err := c.apiClient.Ops.DoUpdate(RKEK8sWindowsSystemImageType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RKEK8sWindowsSystemImageClient) Replace(obj *RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error) {
	resp := &RKEK8sWindowsSystemImage{}
	err := c.apiClient.Ops.DoReplace(RKEK8sWindowsSystemImageType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RKEK8sWindowsSystemImageClient) List(opts *types.ListOpts) (*RKEK8sWindowsSystemImageCollection, error) {
	resp := &RKEK8sWindowsSystemImageCollection{}
	err := c.apiClient.Ops.DoList(RKEK8sWindowsSystemImageType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *RKEK8sWindowsSystemImageCollection) Next() (*RKEK8sWindowsSystemImageCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RKEK8sWindowsSystemImageCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RKEK8sWindowsSystemImageClient) ByID(id string) (*RKEK8sWindowsSystemImage, error) {
	resp := &RKEK8sWindowsSystemImage{}
	err := c.apiClient.Ops.DoByID(RKEK8sWindowsSystemImageType, id, resp)
	return resp, err
}

func (c *RKEK8sWindowsSystemImageClient) Delete(container *RKEK8sWindowsSystemImage) error {
	return c.apiClient.Ops.DoResourceDelete(RKEK8sWindowsSystemImageType, &container.Resource)
}
