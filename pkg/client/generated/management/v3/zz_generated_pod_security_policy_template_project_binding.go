package client

import (
	"github.com/rancher/norman/types"
)

const (
	PodSecurityPolicyTemplateProjectBindingType                               = "podSecurityPolicyTemplateProjectBinding"
	PodSecurityPolicyTemplateProjectBindingFieldAnnotations                   = "annotations"
	PodSecurityPolicyTemplateProjectBindingFieldCreated                       = "created"
	PodSecurityPolicyTemplateProjectBindingFieldCreatorID                     = "creatorId"
	PodSecurityPolicyTemplateProjectBindingFieldLabels                        = "labels"
	PodSecurityPolicyTemplateProjectBindingFieldName                          = "name"
	PodSecurityPolicyTemplateProjectBindingFieldNamespaceId                   = "namespaceId"
	PodSecurityPolicyTemplateProjectBindingFieldOwnerReferences               = "ownerReferences"
	PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName = "podSecurityPolicyTemplateId"
	PodSecurityPolicyTemplateProjectBindingFieldRemoved                       = "removed"
	PodSecurityPolicyTemplateProjectBindingFieldTargetProjectName             = "targetProjectId"
	PodSecurityPolicyTemplateProjectBindingFieldUUID                          = "uuid"
)

type PodSecurityPolicyTemplateProjectBinding struct {
	types.Resource
	Annotations                   map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created                       string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels                        map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                          string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                   string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences               []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PodSecurityPolicyTemplateName string            `json:"podSecurityPolicyTemplateId,omitempty" yaml:"podSecurityPolicyTemplateId,omitempty"`
	Removed                       string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	TargetProjectName             string            `json:"targetProjectId,omitempty" yaml:"targetProjectId,omitempty"`
	UUID                          string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type PodSecurityPolicyTemplateProjectBindingCollection struct {
	types.Collection
	Data   []PodSecurityPolicyTemplateProjectBinding `json:"data,omitempty"`
	client *PodSecurityPolicyTemplateProjectBindingClient
}

type PodSecurityPolicyTemplateProjectBindingClient struct {
	apiClient *Client
}

type PodSecurityPolicyTemplateProjectBindingOperations interface {
	List(opts *types.ListOpts) (*PodSecurityPolicyTemplateProjectBindingCollection, error)
	ListAll(opts *types.ListOpts) (*PodSecurityPolicyTemplateProjectBindingCollection, error)
	Create(opts *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	Update(existing *PodSecurityPolicyTemplateProjectBinding, updates interface{}) (*PodSecurityPolicyTemplateProjectBinding, error)
	Replace(existing *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	ByID(id string) (*PodSecurityPolicyTemplateProjectBinding, error)
	Delete(container *PodSecurityPolicyTemplateProjectBinding) error
}

func newPodSecurityPolicyTemplateProjectBindingClient(apiClient *Client) *PodSecurityPolicyTemplateProjectBindingClient {
	return &PodSecurityPolicyTemplateProjectBindingClient{
		apiClient: apiClient,
	}
}

func (c *PodSecurityPolicyTemplateProjectBindingClient) Create(container *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error) {
	resp := &PodSecurityPolicyTemplateProjectBinding{}
	err := c.apiClient.Ops.DoCreate(PodSecurityPolicyTemplateProjectBindingType, container, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateProjectBindingClient) Update(existing *PodSecurityPolicyTemplateProjectBinding, updates interface{}) (*PodSecurityPolicyTemplateProjectBinding, error) {
	resp := &PodSecurityPolicyTemplateProjectBinding{}
	err := c.apiClient.Ops.DoUpdate(PodSecurityPolicyTemplateProjectBindingType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateProjectBindingClient) Replace(obj *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error) {
	resp := &PodSecurityPolicyTemplateProjectBinding{}
	err := c.apiClient.Ops.DoReplace(PodSecurityPolicyTemplateProjectBindingType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateProjectBindingClient) List(opts *types.ListOpts) (*PodSecurityPolicyTemplateProjectBindingCollection, error) {
	resp := &PodSecurityPolicyTemplateProjectBindingCollection{}
	err := c.apiClient.Ops.DoList(PodSecurityPolicyTemplateProjectBindingType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *PodSecurityPolicyTemplateProjectBindingClient) ListAll(opts *types.ListOpts) (*PodSecurityPolicyTemplateProjectBindingCollection, error) {
	resp := &PodSecurityPolicyTemplateProjectBindingCollection{}
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

func (cc *PodSecurityPolicyTemplateProjectBindingCollection) Next() (*PodSecurityPolicyTemplateProjectBindingCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PodSecurityPolicyTemplateProjectBindingCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PodSecurityPolicyTemplateProjectBindingClient) ByID(id string) (*PodSecurityPolicyTemplateProjectBinding, error) {
	resp := &PodSecurityPolicyTemplateProjectBinding{}
	err := c.apiClient.Ops.DoByID(PodSecurityPolicyTemplateProjectBindingType, id, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateProjectBindingClient) Delete(container *PodSecurityPolicyTemplateProjectBinding) error {
	return c.apiClient.Ops.DoResourceDelete(PodSecurityPolicyTemplateProjectBindingType, &container.Resource)
}
