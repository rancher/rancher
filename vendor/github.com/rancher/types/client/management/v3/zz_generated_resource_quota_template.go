package client

import (
	"github.com/rancher/norman/types"
)

const (
	ResourceQuotaTemplateType                 = "resourceQuotaTemplate"
	ResourceQuotaTemplateFieldAnnotations     = "annotations"
	ResourceQuotaTemplateFieldClusterId       = "clusterId"
	ResourceQuotaTemplateFieldCreated         = "created"
	ResourceQuotaTemplateFieldCreatorID       = "creatorId"
	ResourceQuotaTemplateFieldDescription     = "description"
	ResourceQuotaTemplateFieldIsDefault       = "isDefault"
	ResourceQuotaTemplateFieldLabels          = "labels"
	ResourceQuotaTemplateFieldLimit           = "limit"
	ResourceQuotaTemplateFieldName            = "name"
	ResourceQuotaTemplateFieldNamespaceId     = "namespaceId"
	ResourceQuotaTemplateFieldOwnerReferences = "ownerReferences"
	ResourceQuotaTemplateFieldRemoved         = "removed"
	ResourceQuotaTemplateFieldUsedLimit       = "usedLimit"
	ResourceQuotaTemplateFieldUuid            = "uuid"
)

type ResourceQuotaTemplate struct {
	types.Resource
	Annotations     map[string]string     `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterId       string                `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created         string                `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string                `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description     string                `json:"description,omitempty" yaml:"description,omitempty"`
	IsDefault       bool                  `json:"isDefault,omitempty" yaml:"isDefault,omitempty"`
	Labels          map[string]string     `json:"labels,omitempty" yaml:"labels,omitempty"`
	Limit           *ProjectResourceLimit `json:"limit,omitempty" yaml:"limit,omitempty"`
	Name            string                `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string                `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference      `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string                `json:"removed,omitempty" yaml:"removed,omitempty"`
	UsedLimit       *ProjectResourceLimit `json:"usedLimit,omitempty" yaml:"usedLimit,omitempty"`
	Uuid            string                `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type ResourceQuotaTemplateCollection struct {
	types.Collection
	Data   []ResourceQuotaTemplate `json:"data,omitempty"`
	client *ResourceQuotaTemplateClient
}

type ResourceQuotaTemplateClient struct {
	apiClient *Client
}

type ResourceQuotaTemplateOperations interface {
	List(opts *types.ListOpts) (*ResourceQuotaTemplateCollection, error)
	Create(opts *ResourceQuotaTemplate) (*ResourceQuotaTemplate, error)
	Update(existing *ResourceQuotaTemplate, updates interface{}) (*ResourceQuotaTemplate, error)
	ByID(id string) (*ResourceQuotaTemplate, error)
	Delete(container *ResourceQuotaTemplate) error
}

func newResourceQuotaTemplateClient(apiClient *Client) *ResourceQuotaTemplateClient {
	return &ResourceQuotaTemplateClient{
		apiClient: apiClient,
	}
}

func (c *ResourceQuotaTemplateClient) Create(container *ResourceQuotaTemplate) (*ResourceQuotaTemplate, error) {
	resp := &ResourceQuotaTemplate{}
	err := c.apiClient.Ops.DoCreate(ResourceQuotaTemplateType, container, resp)
	return resp, err
}

func (c *ResourceQuotaTemplateClient) Update(existing *ResourceQuotaTemplate, updates interface{}) (*ResourceQuotaTemplate, error) {
	resp := &ResourceQuotaTemplate{}
	err := c.apiClient.Ops.DoUpdate(ResourceQuotaTemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ResourceQuotaTemplateClient) List(opts *types.ListOpts) (*ResourceQuotaTemplateCollection, error) {
	resp := &ResourceQuotaTemplateCollection{}
	err := c.apiClient.Ops.DoList(ResourceQuotaTemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ResourceQuotaTemplateCollection) Next() (*ResourceQuotaTemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ResourceQuotaTemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ResourceQuotaTemplateClient) ByID(id string) (*ResourceQuotaTemplate, error) {
	resp := &ResourceQuotaTemplate{}
	err := c.apiClient.Ops.DoByID(ResourceQuotaTemplateType, id, resp)
	return resp, err
}

func (c *ResourceQuotaTemplateClient) Delete(container *ResourceQuotaTemplate) error {
	return c.apiClient.Ops.DoResourceDelete(ResourceQuotaTemplateType, &container.Resource)
}
