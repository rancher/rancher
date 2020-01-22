package client

import (
	"github.com/rancher/norman/types"
)

const (
	CisBenchmarkVersionType                 = "cisBenchmarkVersion"
	CisBenchmarkVersionFieldAnnotations     = "annotations"
	CisBenchmarkVersionFieldCreated         = "created"
	CisBenchmarkVersionFieldCreatorID       = "creatorId"
	CisBenchmarkVersionFieldInfo            = "info"
	CisBenchmarkVersionFieldLabels          = "labels"
	CisBenchmarkVersionFieldName            = "name"
	CisBenchmarkVersionFieldOwnerReferences = "ownerReferences"
	CisBenchmarkVersionFieldRemoved         = "removed"
	CisBenchmarkVersionFieldUUID            = "uuid"
)

type CisBenchmarkVersion struct {
	types.Resource
	Annotations     map[string]string        `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string                   `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string                   `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Info            *CisBenchmarkVersionInfo `json:"info,omitempty" yaml:"info,omitempty"`
	Labels          map[string]string        `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string                   `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference         `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string                   `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string                   `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type CisBenchmarkVersionCollection struct {
	types.Collection
	Data   []CisBenchmarkVersion `json:"data,omitempty"`
	client *CisBenchmarkVersionClient
}

type CisBenchmarkVersionClient struct {
	apiClient *Client
}

type CisBenchmarkVersionOperations interface {
	List(opts *types.ListOpts) (*CisBenchmarkVersionCollection, error)
	Create(opts *CisBenchmarkVersion) (*CisBenchmarkVersion, error)
	Update(existing *CisBenchmarkVersion, updates interface{}) (*CisBenchmarkVersion, error)
	Replace(existing *CisBenchmarkVersion) (*CisBenchmarkVersion, error)
	ByID(id string) (*CisBenchmarkVersion, error)
	Delete(container *CisBenchmarkVersion) error
}

func newCisBenchmarkVersionClient(apiClient *Client) *CisBenchmarkVersionClient {
	return &CisBenchmarkVersionClient{
		apiClient: apiClient,
	}
}

func (c *CisBenchmarkVersionClient) Create(container *CisBenchmarkVersion) (*CisBenchmarkVersion, error) {
	resp := &CisBenchmarkVersion{}
	err := c.apiClient.Ops.DoCreate(CisBenchmarkVersionType, container, resp)
	return resp, err
}

func (c *CisBenchmarkVersionClient) Update(existing *CisBenchmarkVersion, updates interface{}) (*CisBenchmarkVersion, error) {
	resp := &CisBenchmarkVersion{}
	err := c.apiClient.Ops.DoUpdate(CisBenchmarkVersionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CisBenchmarkVersionClient) Replace(obj *CisBenchmarkVersion) (*CisBenchmarkVersion, error) {
	resp := &CisBenchmarkVersion{}
	err := c.apiClient.Ops.DoReplace(CisBenchmarkVersionType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *CisBenchmarkVersionClient) List(opts *types.ListOpts) (*CisBenchmarkVersionCollection, error) {
	resp := &CisBenchmarkVersionCollection{}
	err := c.apiClient.Ops.DoList(CisBenchmarkVersionType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *CisBenchmarkVersionCollection) Next() (*CisBenchmarkVersionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CisBenchmarkVersionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CisBenchmarkVersionClient) ByID(id string) (*CisBenchmarkVersion, error) {
	resp := &CisBenchmarkVersion{}
	err := c.apiClient.Ops.DoByID(CisBenchmarkVersionType, id, resp)
	return resp, err
}

func (c *CisBenchmarkVersionClient) Delete(container *CisBenchmarkVersion) error {
	return c.apiClient.Ops.DoResourceDelete(CisBenchmarkVersionType, &container.Resource)
}
