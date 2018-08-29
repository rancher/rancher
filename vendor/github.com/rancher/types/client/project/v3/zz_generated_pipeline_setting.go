package client

import (
	"github.com/rancher/norman/types"
)

const (
	PipelineSettingType                 = "pipelineSetting"
	PipelineSettingFieldAnnotations     = "annotations"
	PipelineSettingFieldCreated         = "created"
	PipelineSettingFieldCreatorID       = "creatorId"
	PipelineSettingFieldCustomized      = "customized"
	PipelineSettingFieldDefault         = "default"
	PipelineSettingFieldLabels          = "labels"
	PipelineSettingFieldName            = "name"
	PipelineSettingFieldNamespaceId     = "namespaceId"
	PipelineSettingFieldOwnerReferences = "ownerReferences"
	PipelineSettingFieldProjectID       = "projectId"
	PipelineSettingFieldRemoved         = "removed"
	PipelineSettingFieldUUID            = "uuid"
	PipelineSettingFieldValue           = "value"
)

type PipelineSetting struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Customized      bool              `json:"customized,omitempty" yaml:"customized,omitempty"`
	Default         string            `json:"default,omitempty" yaml:"default,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Value           string            `json:"value,omitempty" yaml:"value,omitempty"`
}

type PipelineSettingCollection struct {
	types.Collection
	Data   []PipelineSetting `json:"data,omitempty"`
	client *PipelineSettingClient
}

type PipelineSettingClient struct {
	apiClient *Client
}

type PipelineSettingOperations interface {
	List(opts *types.ListOpts) (*PipelineSettingCollection, error)
	Create(opts *PipelineSetting) (*PipelineSetting, error)
	Update(existing *PipelineSetting, updates interface{}) (*PipelineSetting, error)
	Replace(existing *PipelineSetting) (*PipelineSetting, error)
	ByID(id string) (*PipelineSetting, error)
	Delete(container *PipelineSetting) error
}

func newPipelineSettingClient(apiClient *Client) *PipelineSettingClient {
	return &PipelineSettingClient{
		apiClient: apiClient,
	}
}

func (c *PipelineSettingClient) Create(container *PipelineSetting) (*PipelineSetting, error) {
	resp := &PipelineSetting{}
	err := c.apiClient.Ops.DoCreate(PipelineSettingType, container, resp)
	return resp, err
}

func (c *PipelineSettingClient) Update(existing *PipelineSetting, updates interface{}) (*PipelineSetting, error) {
	resp := &PipelineSetting{}
	err := c.apiClient.Ops.DoUpdate(PipelineSettingType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PipelineSettingClient) Replace(obj *PipelineSetting) (*PipelineSetting, error) {
	resp := &PipelineSetting{}
	err := c.apiClient.Ops.DoReplace(PipelineSettingType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PipelineSettingClient) List(opts *types.ListOpts) (*PipelineSettingCollection, error) {
	resp := &PipelineSettingCollection{}
	err := c.apiClient.Ops.DoList(PipelineSettingType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *PipelineSettingCollection) Next() (*PipelineSettingCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PipelineSettingCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PipelineSettingClient) ByID(id string) (*PipelineSetting, error) {
	resp := &PipelineSetting{}
	err := c.apiClient.Ops.DoByID(PipelineSettingType, id, resp)
	return resp, err
}

func (c *PipelineSettingClient) Delete(container *PipelineSetting) error {
	return c.apiClient.Ops.DoResourceDelete(PipelineSettingType, &container.Resource)
}
