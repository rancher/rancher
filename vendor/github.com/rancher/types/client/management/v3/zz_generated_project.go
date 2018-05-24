package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectType                               = "project"
	ProjectFieldAnnotations                   = "annotations"
	ProjectFieldClusterId                     = "clusterId"
	ProjectFieldConditions                    = "conditions"
	ProjectFieldCreated                       = "created"
	ProjectFieldCreatorID                     = "creatorId"
	ProjectFieldDescription                   = "description"
	ProjectFieldLabels                        = "labels"
	ProjectFieldName                          = "name"
	ProjectFieldNamespaceId                   = "namespaceId"
	ProjectFieldOwnerReferences               = "ownerReferences"
	ProjectFieldPodSecurityPolicyTemplateName = "podSecurityPolicyTemplateId"
	ProjectFieldRemoved                       = "removed"
	ProjectFieldState                         = "state"
	ProjectFieldTransitioning                 = "transitioning"
	ProjectFieldTransitioningMessage          = "transitioningMessage"
	ProjectFieldUuid                          = "uuid"
)

type Project struct {
	types.Resource
	Annotations                   map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterId                     string             `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Conditions                    []ProjectCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created                       string             `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string             `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description                   string             `json:"description,omitempty" yaml:"description,omitempty"`
	Labels                        map[string]string  `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                          string             `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                   string             `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences               []OwnerReference   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PodSecurityPolicyTemplateName string             `json:"podSecurityPolicyTemplateId,omitempty" yaml:"podSecurityPolicyTemplateId,omitempty"`
	Removed                       string             `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                         string             `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning                 string             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage          string             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Uuid                          string             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type ProjectCollection struct {
	types.Collection
	Data   []Project `json:"data,omitempty"`
	client *ProjectClient
}

type ProjectClient struct {
	apiClient *Client
}

type ProjectOperations interface {
	List(opts *types.ListOpts) (*ProjectCollection, error)
	Create(opts *Project) (*Project, error)
	Update(existing *Project, updates interface{}) (*Project, error)
	ByID(id string) (*Project, error)
	Delete(container *Project) error

	ActionExportYaml(resource *Project) error

	ActionSetpodsecuritypolicytemplate(resource *Project, input *SetPodSecurityPolicyTemplateInput) (*Project, error)
}

func newProjectClient(apiClient *Client) *ProjectClient {
	return &ProjectClient{
		apiClient: apiClient,
	}
}

func (c *ProjectClient) Create(container *Project) (*Project, error) {
	resp := &Project{}
	err := c.apiClient.Ops.DoCreate(ProjectType, container, resp)
	return resp, err
}

func (c *ProjectClient) Update(existing *Project, updates interface{}) (*Project, error) {
	resp := &Project{}
	err := c.apiClient.Ops.DoUpdate(ProjectType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectClient) List(opts *types.ListOpts) (*ProjectCollection, error) {
	resp := &ProjectCollection{}
	err := c.apiClient.Ops.DoList(ProjectType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ProjectCollection) Next() (*ProjectCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectClient) ByID(id string) (*Project, error) {
	resp := &Project{}
	err := c.apiClient.Ops.DoByID(ProjectType, id, resp)
	return resp, err
}

func (c *ProjectClient) Delete(container *Project) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectType, &container.Resource)
}

func (c *ProjectClient) ActionExportYaml(resource *Project) error {
	err := c.apiClient.Ops.DoAction(ProjectType, "exportYaml", &resource.Resource, nil, nil)
	return err
}

func (c *ProjectClient) ActionSetpodsecuritypolicytemplate(resource *Project, input *SetPodSecurityPolicyTemplateInput) (*Project, error) {
	resp := &Project{}
	err := c.apiClient.Ops.DoAction(ProjectType, "setpodsecuritypolicytemplate", &resource.Resource, input, resp)
	return resp, err
}
