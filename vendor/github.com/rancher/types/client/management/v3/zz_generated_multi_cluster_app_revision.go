package client

import (
	"github.com/rancher/norman/types"
)

const (
	MultiClusterAppRevisionType                   = "multiClusterAppRevision"
	MultiClusterAppRevisionFieldAnnotations       = "annotations"
	MultiClusterAppRevisionFieldAnswers           = "answers"
	MultiClusterAppRevisionFieldCreated           = "created"
	MultiClusterAppRevisionFieldCreatorID         = "creatorId"
	MultiClusterAppRevisionFieldLabels            = "labels"
	MultiClusterAppRevisionFieldName              = "name"
	MultiClusterAppRevisionFieldOwnerReferences   = "ownerReferences"
	MultiClusterAppRevisionFieldRemoved           = "removed"
	MultiClusterAppRevisionFieldTemplateVersionID = "templateVersionId"
	MultiClusterAppRevisionFieldUUID              = "uuid"
)

type MultiClusterAppRevision struct {
	types.Resource
	Annotations       map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Answers           []Answer          `json:"answers,omitempty" yaml:"answers,omitempty"`
	Created           string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID         string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels            map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name              string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences   []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed           string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	TemplateVersionID string            `json:"templateVersionId,omitempty" yaml:"templateVersionId,omitempty"`
	UUID              string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type MultiClusterAppRevisionCollection struct {
	types.Collection
	Data   []MultiClusterAppRevision `json:"data,omitempty"`
	client *MultiClusterAppRevisionClient
}

type MultiClusterAppRevisionClient struct {
	apiClient *Client
}

type MultiClusterAppRevisionOperations interface {
	List(opts *types.ListOpts) (*MultiClusterAppRevisionCollection, error)
	ListAll(opts *types.ListOpts) (*MultiClusterAppRevisionCollection, error)
	Create(opts *MultiClusterAppRevision) (*MultiClusterAppRevision, error)
	Update(existing *MultiClusterAppRevision, updates interface{}) (*MultiClusterAppRevision, error)
	Replace(existing *MultiClusterAppRevision) (*MultiClusterAppRevision, error)
	ByID(id string) (*MultiClusterAppRevision, error)
	Delete(container *MultiClusterAppRevision) error
}

func newMultiClusterAppRevisionClient(apiClient *Client) *MultiClusterAppRevisionClient {
	return &MultiClusterAppRevisionClient{
		apiClient: apiClient,
	}
}

func (c *MultiClusterAppRevisionClient) Create(container *MultiClusterAppRevision) (*MultiClusterAppRevision, error) {
	resp := &MultiClusterAppRevision{}
	err := c.apiClient.Ops.DoCreate(MultiClusterAppRevisionType, container, resp)
	return resp, err
}

func (c *MultiClusterAppRevisionClient) Update(existing *MultiClusterAppRevision, updates interface{}) (*MultiClusterAppRevision, error) {
	resp := &MultiClusterAppRevision{}
	err := c.apiClient.Ops.DoUpdate(MultiClusterAppRevisionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *MultiClusterAppRevisionClient) Replace(obj *MultiClusterAppRevision) (*MultiClusterAppRevision, error) {
	resp := &MultiClusterAppRevision{}
	err := c.apiClient.Ops.DoReplace(MultiClusterAppRevisionType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *MultiClusterAppRevisionClient) List(opts *types.ListOpts) (*MultiClusterAppRevisionCollection, error) {
	resp := &MultiClusterAppRevisionCollection{}
	err := c.apiClient.Ops.DoList(MultiClusterAppRevisionType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *MultiClusterAppRevisionClient) ListAll(opts *types.ListOpts) (*MultiClusterAppRevisionCollection, error) {
	resp := &MultiClusterAppRevisionCollection{}
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

func (cc *MultiClusterAppRevisionCollection) Next() (*MultiClusterAppRevisionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &MultiClusterAppRevisionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *MultiClusterAppRevisionClient) ByID(id string) (*MultiClusterAppRevision, error) {
	resp := &MultiClusterAppRevision{}
	err := c.apiClient.Ops.DoByID(MultiClusterAppRevisionType, id, resp)
	return resp, err
}

func (c *MultiClusterAppRevisionClient) Delete(container *MultiClusterAppRevision) error {
	return c.apiClient.Ops.DoResourceDelete(MultiClusterAppRevisionType, &container.Resource)
}
