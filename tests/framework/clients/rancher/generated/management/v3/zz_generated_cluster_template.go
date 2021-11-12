package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterTemplateType                   = "clusterTemplate"
	ClusterTemplateFieldAnnotations       = "annotations"
	ClusterTemplateFieldCreated           = "created"
	ClusterTemplateFieldCreatorID         = "creatorId"
	ClusterTemplateFieldDefaultRevisionID = "defaultRevisionId"
	ClusterTemplateFieldDescription       = "description"
	ClusterTemplateFieldLabels            = "labels"
	ClusterTemplateFieldMembers           = "members"
	ClusterTemplateFieldName              = "name"
	ClusterTemplateFieldOwnerReferences   = "ownerReferences"
	ClusterTemplateFieldRemoved           = "removed"
	ClusterTemplateFieldUUID              = "uuid"
)

type ClusterTemplate struct {
	types.Resource
	Annotations       map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created           string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID         string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DefaultRevisionID string            `json:"defaultRevisionId,omitempty" yaml:"defaultRevisionId,omitempty"`
	Description       string            `json:"description,omitempty" yaml:"description,omitempty"`
	Labels            map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Members           []Member          `json:"members,omitempty" yaml:"members,omitempty"`
	Name              string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences   []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed           string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID              string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ClusterTemplateCollection struct {
	types.Collection
	Data   []ClusterTemplate `json:"data,omitempty"`
	client *ClusterTemplateClient
}

type ClusterTemplateClient struct {
	apiClient *Client
}

type ClusterTemplateOperations interface {
	List(opts *types.ListOpts) (*ClusterTemplateCollection, error)
	ListAll(opts *types.ListOpts) (*ClusterTemplateCollection, error)
	Create(opts *ClusterTemplate) (*ClusterTemplate, error)
	Update(existing *ClusterTemplate, updates interface{}) (*ClusterTemplate, error)
	Replace(existing *ClusterTemplate) (*ClusterTemplate, error)
	ByID(id string) (*ClusterTemplate, error)
	Delete(container *ClusterTemplate) error
}

func newClusterTemplateClient(apiClient *Client) *ClusterTemplateClient {
	return &ClusterTemplateClient{
		apiClient: apiClient,
	}
}

func (c *ClusterTemplateClient) Create(container *ClusterTemplate) (*ClusterTemplate, error) {
	resp := &ClusterTemplate{}
	err := c.apiClient.Ops.DoCreate(ClusterTemplateType, container, resp)
	return resp, err
}

func (c *ClusterTemplateClient) Update(existing *ClusterTemplate, updates interface{}) (*ClusterTemplate, error) {
	resp := &ClusterTemplate{}
	err := c.apiClient.Ops.DoUpdate(ClusterTemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterTemplateClient) Replace(obj *ClusterTemplate) (*ClusterTemplate, error) {
	resp := &ClusterTemplate{}
	err := c.apiClient.Ops.DoReplace(ClusterTemplateType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterTemplateClient) List(opts *types.ListOpts) (*ClusterTemplateCollection, error) {
	resp := &ClusterTemplateCollection{}
	err := c.apiClient.Ops.DoList(ClusterTemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ClusterTemplateClient) ListAll(opts *types.ListOpts) (*ClusterTemplateCollection, error) {
	resp := &ClusterTemplateCollection{}
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

func (cc *ClusterTemplateCollection) Next() (*ClusterTemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterTemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterTemplateClient) ByID(id string) (*ClusterTemplate, error) {
	resp := &ClusterTemplate{}
	err := c.apiClient.Ops.DoByID(ClusterTemplateType, id, resp)
	return resp, err
}

func (c *ClusterTemplateClient) Delete(container *ClusterTemplate) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterTemplateType, &container.Resource)
}
