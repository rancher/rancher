package client

import (
	"github.com/rancher/norman/types"
)

const (
	PodSecurityPolicyTemplateType                 = "podSecurityPolicyTemplate"
	PodSecurityPolicyTemplateFieldAnnotations     = "annotations"
	PodSecurityPolicyTemplateFieldCreated         = "created"
	PodSecurityPolicyTemplateFieldCreatorID       = "creatorId"
	PodSecurityPolicyTemplateFieldDescription     = "description"
	PodSecurityPolicyTemplateFieldLabels          = "labels"
	PodSecurityPolicyTemplateFieldName            = "name"
	PodSecurityPolicyTemplateFieldOwnerReferences = "ownerReferences"
	PodSecurityPolicyTemplateFieldRemoved         = "removed"
	PodSecurityPolicyTemplateFieldUUID            = "uuid"
)

type PodSecurityPolicyTemplate struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type PodSecurityPolicyTemplateCollection struct {
	types.Collection
	Data   []PodSecurityPolicyTemplate `json:"data,omitempty"`
	client *PodSecurityPolicyTemplateClient
}

type PodSecurityPolicyTemplateClient struct {
	apiClient *Client
}

type PodSecurityPolicyTemplateOperations interface {
	List(opts *types.ListOpts) (*PodSecurityPolicyTemplateCollection, error)
	ListAll(opts *types.ListOpts) (*PodSecurityPolicyTemplateCollection, error)
	Create(opts *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Update(existing *PodSecurityPolicyTemplate, updates interface{}) (*PodSecurityPolicyTemplate, error)
	Replace(existing *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	ByID(id string) (*PodSecurityPolicyTemplate, error)
	Delete(container *PodSecurityPolicyTemplate) error
}

func newPodSecurityPolicyTemplateClient(apiClient *Client) *PodSecurityPolicyTemplateClient {
	return &PodSecurityPolicyTemplateClient{
		apiClient: apiClient,
	}
}

func (c *PodSecurityPolicyTemplateClient) Create(container *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	resp := &PodSecurityPolicyTemplate{}
	err := c.apiClient.Ops.DoCreate(PodSecurityPolicyTemplateType, container, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) Update(existing *PodSecurityPolicyTemplate, updates interface{}) (*PodSecurityPolicyTemplate, error) {
	resp := &PodSecurityPolicyTemplate{}
	err := c.apiClient.Ops.DoUpdate(PodSecurityPolicyTemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) Replace(obj *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	resp := &PodSecurityPolicyTemplate{}
	err := c.apiClient.Ops.DoReplace(PodSecurityPolicyTemplateType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) List(opts *types.ListOpts) (*PodSecurityPolicyTemplateCollection, error) {
	resp := &PodSecurityPolicyTemplateCollection{}
	err := c.apiClient.Ops.DoList(PodSecurityPolicyTemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) ListAll(opts *types.ListOpts) (*PodSecurityPolicyTemplateCollection, error) {
	resp := &PodSecurityPolicyTemplateCollection{}
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

func (cc *PodSecurityPolicyTemplateCollection) Next() (*PodSecurityPolicyTemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PodSecurityPolicyTemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PodSecurityPolicyTemplateClient) ByID(id string) (*PodSecurityPolicyTemplate, error) {
	resp := &PodSecurityPolicyTemplate{}
	err := c.apiClient.Ops.DoByID(PodSecurityPolicyTemplateType, id, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) Delete(container *PodSecurityPolicyTemplate) error {
	return c.apiClient.Ops.DoResourceDelete(PodSecurityPolicyTemplateType, &container.Resource)
}
