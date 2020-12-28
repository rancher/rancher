package client

import (
	"github.com/rancher/norman/types"
)

const (
	TemplateContentType                 = "templateContent"
	TemplateContentFieldAnnotations     = "annotations"
	TemplateContentFieldCreated         = "created"
	TemplateContentFieldCreatorID       = "creatorId"
	TemplateContentFieldData            = "data"
	TemplateContentFieldLabels          = "labels"
	TemplateContentFieldName            = "name"
	TemplateContentFieldOwnerReferences = "ownerReferences"
	TemplateContentFieldRemoved         = "removed"
	TemplateContentFieldUUID            = "uuid"
)

type TemplateContent struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Data            string            `json:"data,omitempty" yaml:"data,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type TemplateContentCollection struct {
	types.Collection
	Data   []TemplateContent `json:"data,omitempty"`
	client *TemplateContentClient
}

type TemplateContentClient struct {
	apiClient *Client
}

type TemplateContentOperations interface {
	List(opts *types.ListOpts) (*TemplateContentCollection, error)
	ListAll(opts *types.ListOpts) (*TemplateContentCollection, error)
	Create(opts *TemplateContent) (*TemplateContent, error)
	Update(existing *TemplateContent, updates interface{}) (*TemplateContent, error)
	Replace(existing *TemplateContent) (*TemplateContent, error)
	ByID(id string) (*TemplateContent, error)
	Delete(container *TemplateContent) error
}

func newTemplateContentClient(apiClient *Client) *TemplateContentClient {
	return &TemplateContentClient{
		apiClient: apiClient,
	}
}

func (c *TemplateContentClient) Create(container *TemplateContent) (*TemplateContent, error) {
	resp := &TemplateContent{}
	err := c.apiClient.Ops.DoCreate(TemplateContentType, container, resp)
	return resp, err
}

func (c *TemplateContentClient) Update(existing *TemplateContent, updates interface{}) (*TemplateContent, error) {
	resp := &TemplateContent{}
	err := c.apiClient.Ops.DoUpdate(TemplateContentType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *TemplateContentClient) Replace(obj *TemplateContent) (*TemplateContent, error) {
	resp := &TemplateContent{}
	err := c.apiClient.Ops.DoReplace(TemplateContentType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *TemplateContentClient) List(opts *types.ListOpts) (*TemplateContentCollection, error) {
	resp := &TemplateContentCollection{}
	err := c.apiClient.Ops.DoList(TemplateContentType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *TemplateContentClient) ListAll(opts *types.ListOpts) (*TemplateContentCollection, error) {
	resp := &TemplateContentCollection{}
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

func (cc *TemplateContentCollection) Next() (*TemplateContentCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &TemplateContentCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *TemplateContentClient) ByID(id string) (*TemplateContent, error) {
	resp := &TemplateContent{}
	err := c.apiClient.Ops.DoByID(TemplateContentType, id, resp)
	return resp, err
}

func (c *TemplateContentClient) Delete(container *TemplateContent) error {
	return c.apiClient.Ops.DoResourceDelete(TemplateContentType, &container.Resource)
}
