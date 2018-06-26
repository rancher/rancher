package client

import (
	"github.com/rancher/norman/types"
)

const (
	SourceCodeProviderType                 = "sourceCodeProvider"
	SourceCodeProviderFieldAnnotations     = "annotations"
	SourceCodeProviderFieldCreated         = "created"
	SourceCodeProviderFieldCreatorID       = "creatorId"
	SourceCodeProviderFieldLabels          = "labels"
	SourceCodeProviderFieldName            = "name"
	SourceCodeProviderFieldOwnerReferences = "ownerReferences"
	SourceCodeProviderFieldProjectID       = "projectId"
	SourceCodeProviderFieldRemoved         = "removed"
	SourceCodeProviderFieldType            = "type"
	SourceCodeProviderFieldUUID            = "uuid"
)

type SourceCodeProvider struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type SourceCodeProviderCollection struct {
	types.Collection
	Data   []SourceCodeProvider `json:"data,omitempty"`
	client *SourceCodeProviderClient
}

type SourceCodeProviderClient struct {
	apiClient *Client
}

type SourceCodeProviderOperations interface {
	List(opts *types.ListOpts) (*SourceCodeProviderCollection, error)
	Create(opts *SourceCodeProvider) (*SourceCodeProvider, error)
	Update(existing *SourceCodeProvider, updates interface{}) (*SourceCodeProvider, error)
	Replace(existing *SourceCodeProvider) (*SourceCodeProvider, error)
	ByID(id string) (*SourceCodeProvider, error)
	Delete(container *SourceCodeProvider) error
}

func newSourceCodeProviderClient(apiClient *Client) *SourceCodeProviderClient {
	return &SourceCodeProviderClient{
		apiClient: apiClient,
	}
}

func (c *SourceCodeProviderClient) Create(container *SourceCodeProvider) (*SourceCodeProvider, error) {
	resp := &SourceCodeProvider{}
	err := c.apiClient.Ops.DoCreate(SourceCodeProviderType, container, resp)
	return resp, err
}

func (c *SourceCodeProviderClient) Update(existing *SourceCodeProvider, updates interface{}) (*SourceCodeProvider, error) {
	resp := &SourceCodeProvider{}
	err := c.apiClient.Ops.DoUpdate(SourceCodeProviderType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *SourceCodeProviderClient) Replace(obj *SourceCodeProvider) (*SourceCodeProvider, error) {
	resp := &SourceCodeProvider{}
	err := c.apiClient.Ops.DoReplace(SourceCodeProviderType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *SourceCodeProviderClient) List(opts *types.ListOpts) (*SourceCodeProviderCollection, error) {
	resp := &SourceCodeProviderCollection{}
	err := c.apiClient.Ops.DoList(SourceCodeProviderType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *SourceCodeProviderCollection) Next() (*SourceCodeProviderCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &SourceCodeProviderCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *SourceCodeProviderClient) ByID(id string) (*SourceCodeProvider, error) {
	resp := &SourceCodeProvider{}
	err := c.apiClient.Ops.DoByID(SourceCodeProviderType, id, resp)
	return resp, err
}

func (c *SourceCodeProviderClient) Delete(container *SourceCodeProvider) error {
	return c.apiClient.Ops.DoResourceDelete(SourceCodeProviderType, &container.Resource)
}
