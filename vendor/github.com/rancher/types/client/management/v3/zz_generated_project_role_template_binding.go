package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectRoleTemplateBindingType                  = "projectRoleTemplateBinding"
	ProjectRoleTemplateBindingFieldAnnotations      = "annotations"
	ProjectRoleTemplateBindingFieldCreated          = "created"
	ProjectRoleTemplateBindingFieldCreatorID        = "creatorId"
	ProjectRoleTemplateBindingFieldLabels           = "labels"
	ProjectRoleTemplateBindingFieldName             = "name"
	ProjectRoleTemplateBindingFieldNamespaceId      = "namespaceId"
	ProjectRoleTemplateBindingFieldOwnerReferences  = "ownerReferences"
	ProjectRoleTemplateBindingFieldProjectId        = "projectId"
	ProjectRoleTemplateBindingFieldRemoved          = "removed"
	ProjectRoleTemplateBindingFieldRoleTemplateId   = "roleTemplateId"
	ProjectRoleTemplateBindingFieldSubjectKind      = "subjectKind"
	ProjectRoleTemplateBindingFieldSubjectName      = "subjectName"
	ProjectRoleTemplateBindingFieldSubjectNamespace = "subjectNamespace"
	ProjectRoleTemplateBindingFieldUuid             = "uuid"
)

type ProjectRoleTemplateBinding struct {
	types.Resource
	Annotations      map[string]string `json:"annotations,omitempty"`
	Created          string            `json:"created,omitempty"`
	CreatorID        string            `json:"creatorId,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Name             string            `json:"name,omitempty"`
	NamespaceId      string            `json:"namespaceId,omitempty"`
	OwnerReferences  []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectId        string            `json:"projectId,omitempty"`
	Removed          string            `json:"removed,omitempty"`
	RoleTemplateId   string            `json:"roleTemplateId,omitempty"`
	SubjectKind      string            `json:"subjectKind,omitempty"`
	SubjectName      string            `json:"subjectName,omitempty"`
	SubjectNamespace string            `json:"subjectNamespace,omitempty"`
	Uuid             string            `json:"uuid,omitempty"`
}
type ProjectRoleTemplateBindingCollection struct {
	types.Collection
	Data   []ProjectRoleTemplateBinding `json:"data,omitempty"`
	client *ProjectRoleTemplateBindingClient
}

type ProjectRoleTemplateBindingClient struct {
	apiClient *Client
}

type ProjectRoleTemplateBindingOperations interface {
	List(opts *types.ListOpts) (*ProjectRoleTemplateBindingCollection, error)
	Create(opts *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error)
	Update(existing *ProjectRoleTemplateBinding, updates interface{}) (*ProjectRoleTemplateBinding, error)
	ByID(id string) (*ProjectRoleTemplateBinding, error)
	Delete(container *ProjectRoleTemplateBinding) error
}

func newProjectRoleTemplateBindingClient(apiClient *Client) *ProjectRoleTemplateBindingClient {
	return &ProjectRoleTemplateBindingClient{
		apiClient: apiClient,
	}
}

func (c *ProjectRoleTemplateBindingClient) Create(container *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error) {
	resp := &ProjectRoleTemplateBinding{}
	err := c.apiClient.Ops.DoCreate(ProjectRoleTemplateBindingType, container, resp)
	return resp, err
}

func (c *ProjectRoleTemplateBindingClient) Update(existing *ProjectRoleTemplateBinding, updates interface{}) (*ProjectRoleTemplateBinding, error) {
	resp := &ProjectRoleTemplateBinding{}
	err := c.apiClient.Ops.DoUpdate(ProjectRoleTemplateBindingType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectRoleTemplateBindingClient) List(opts *types.ListOpts) (*ProjectRoleTemplateBindingCollection, error) {
	resp := &ProjectRoleTemplateBindingCollection{}
	err := c.apiClient.Ops.DoList(ProjectRoleTemplateBindingType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ProjectRoleTemplateBindingCollection) Next() (*ProjectRoleTemplateBindingCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectRoleTemplateBindingCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectRoleTemplateBindingClient) ByID(id string) (*ProjectRoleTemplateBinding, error) {
	resp := &ProjectRoleTemplateBinding{}
	err := c.apiClient.Ops.DoByID(ProjectRoleTemplateBindingType, id, resp)
	return resp, err
}

func (c *ProjectRoleTemplateBindingClient) Delete(container *ProjectRoleTemplateBinding) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectRoleTemplateBindingType, &container.Resource)
}
