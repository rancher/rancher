package client

import (
	"github.com/rancher/norman/types"
)

const (
	PodSecurityAdmissionConfigurationTemplateType                 = "podSecurityAdmissionConfigurationTemplate"
	PodSecurityAdmissionConfigurationTemplateFieldAnnotations     = "annotations"
	PodSecurityAdmissionConfigurationTemplateFieldConfiguration   = "configuration"
	PodSecurityAdmissionConfigurationTemplateFieldCreated         = "created"
	PodSecurityAdmissionConfigurationTemplateFieldCreatorID       = "creatorId"
	PodSecurityAdmissionConfigurationTemplateFieldDescription     = "description"
	PodSecurityAdmissionConfigurationTemplateFieldLabels          = "labels"
	PodSecurityAdmissionConfigurationTemplateFieldName            = "name"
	PodSecurityAdmissionConfigurationTemplateFieldOwnerReferences = "ownerReferences"
	PodSecurityAdmissionConfigurationTemplateFieldRemoved         = "removed"
	PodSecurityAdmissionConfigurationTemplateFieldUUID            = "uuid"
)

type PodSecurityAdmissionConfigurationTemplate struct {
	types.Resource
	Annotations     map[string]string                              `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Configuration   *PodSecurityAdmissionConfigurationTemplateSpec `json:"configuration,omitempty" yaml:"configuration,omitempty"`
	Created         string                                         `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string                                         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description     string                                         `json:"description,omitempty" yaml:"description,omitempty"`
	Labels          map[string]string                              `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string                                         `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference                               `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string                                         `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string                                         `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type PodSecurityAdmissionConfigurationTemplateCollection struct {
	types.Collection
	Data   []PodSecurityAdmissionConfigurationTemplate `json:"data,omitempty"`
	client *PodSecurityAdmissionConfigurationTemplateClient
}

type PodSecurityAdmissionConfigurationTemplateClient struct {
	apiClient *Client
}

type PodSecurityAdmissionConfigurationTemplateOperations interface {
	List(opts *types.ListOpts) (*PodSecurityAdmissionConfigurationTemplateCollection, error)
	ListAll(opts *types.ListOpts) (*PodSecurityAdmissionConfigurationTemplateCollection, error)
	Create(opts *PodSecurityAdmissionConfigurationTemplate) (*PodSecurityAdmissionConfigurationTemplate, error)
	Update(existing *PodSecurityAdmissionConfigurationTemplate, updates interface{}) (*PodSecurityAdmissionConfigurationTemplate, error)
	Replace(existing *PodSecurityAdmissionConfigurationTemplate) (*PodSecurityAdmissionConfigurationTemplate, error)
	ByID(id string) (*PodSecurityAdmissionConfigurationTemplate, error)
	Delete(container *PodSecurityAdmissionConfigurationTemplate) error
}

func newPodSecurityAdmissionConfigurationTemplateClient(apiClient *Client) *PodSecurityAdmissionConfigurationTemplateClient {
	return &PodSecurityAdmissionConfigurationTemplateClient{
		apiClient: apiClient,
	}
}

func (c *PodSecurityAdmissionConfigurationTemplateClient) Create(container *PodSecurityAdmissionConfigurationTemplate) (*PodSecurityAdmissionConfigurationTemplate, error) {
	resp := &PodSecurityAdmissionConfigurationTemplate{}
	err := c.apiClient.Ops.DoCreate(PodSecurityAdmissionConfigurationTemplateType, container, resp)
	return resp, err
}

func (c *PodSecurityAdmissionConfigurationTemplateClient) Update(existing *PodSecurityAdmissionConfigurationTemplate, updates interface{}) (*PodSecurityAdmissionConfigurationTemplate, error) {
	resp := &PodSecurityAdmissionConfigurationTemplate{}
	err := c.apiClient.Ops.DoUpdate(PodSecurityAdmissionConfigurationTemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PodSecurityAdmissionConfigurationTemplateClient) Replace(obj *PodSecurityAdmissionConfigurationTemplate) (*PodSecurityAdmissionConfigurationTemplate, error) {
	resp := &PodSecurityAdmissionConfigurationTemplate{}
	err := c.apiClient.Ops.DoReplace(PodSecurityAdmissionConfigurationTemplateType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PodSecurityAdmissionConfigurationTemplateClient) List(opts *types.ListOpts) (*PodSecurityAdmissionConfigurationTemplateCollection, error) {
	resp := &PodSecurityAdmissionConfigurationTemplateCollection{}
	err := c.apiClient.Ops.DoList(PodSecurityAdmissionConfigurationTemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *PodSecurityAdmissionConfigurationTemplateClient) ListAll(opts *types.ListOpts) (*PodSecurityAdmissionConfigurationTemplateCollection, error) {
	resp := &PodSecurityAdmissionConfigurationTemplateCollection{}
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

func (cc *PodSecurityAdmissionConfigurationTemplateCollection) Next() (*PodSecurityAdmissionConfigurationTemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PodSecurityAdmissionConfigurationTemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PodSecurityAdmissionConfigurationTemplateClient) ByID(id string) (*PodSecurityAdmissionConfigurationTemplate, error) {
	resp := &PodSecurityAdmissionConfigurationTemplate{}
	err := c.apiClient.Ops.DoByID(PodSecurityAdmissionConfigurationTemplateType, id, resp)
	return resp, err
}

func (c *PodSecurityAdmissionConfigurationTemplateClient) Delete(container *PodSecurityAdmissionConfigurationTemplate) error {
	return c.apiClient.Ops.DoResourceDelete(PodSecurityAdmissionConfigurationTemplateType, &container.Resource)
}
