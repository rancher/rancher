package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectRoleTemplateBindingType                  = "projectRoleTemplateBinding"
	ProjectRoleTemplateBindingFieldAnnotations      = "annotations"
	ProjectRoleTemplateBindingFieldCreated          = "created"
	ProjectRoleTemplateBindingFieldCreatorID        = "creatorId"
	ProjectRoleTemplateBindingFieldGroupID          = "groupId"
	ProjectRoleTemplateBindingFieldGroupPrincipalID = "groupPrincipalId"
	ProjectRoleTemplateBindingFieldLabels           = "labels"
	ProjectRoleTemplateBindingFieldName             = "name"
	ProjectRoleTemplateBindingFieldNamespaceId      = "namespaceId"
	ProjectRoleTemplateBindingFieldOwnerReferences  = "ownerReferences"
	ProjectRoleTemplateBindingFieldProjectID        = "projectId"
	ProjectRoleTemplateBindingFieldRemoved          = "removed"
	ProjectRoleTemplateBindingFieldRoleTemplateID   = "roleTemplateId"
	ProjectRoleTemplateBindingFieldServiceAccount   = "serviceAccount"
	ProjectRoleTemplateBindingFieldUUID             = "uuid"
	ProjectRoleTemplateBindingFieldUserID           = "userId"
	ProjectRoleTemplateBindingFieldUserPrincipalID  = "userPrincipalId"
)

type ProjectRoleTemplateBinding struct {
	types.Resource
	Annotations      map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created          string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID        string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	GroupID          string            `json:"groupId,omitempty" yaml:"groupId,omitempty"`
	GroupPrincipalID string            `json:"groupPrincipalId,omitempty" yaml:"groupPrincipalId,omitempty"`
	Labels           map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name             string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId      string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences  []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID        string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed          string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	RoleTemplateID   string            `json:"roleTemplateId,omitempty" yaml:"roleTemplateId,omitempty"`
	ServiceAccount   string            `json:"serviceAccount,omitempty" yaml:"serviceAccount,omitempty"`
	UUID             string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserID           string            `json:"userId,omitempty" yaml:"userId,omitempty"`
	UserPrincipalID  string            `json:"userPrincipalId,omitempty" yaml:"userPrincipalId,omitempty"`
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
	ListAll(opts *types.ListOpts) (*ProjectRoleTemplateBindingCollection, error)
	Create(opts *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error)
	Update(existing *ProjectRoleTemplateBinding, updates interface{}) (*ProjectRoleTemplateBinding, error)
	Replace(existing *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error)
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

func (c *ProjectRoleTemplateBindingClient) Replace(obj *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error) {
	resp := &ProjectRoleTemplateBinding{}
	err := c.apiClient.Ops.DoReplace(ProjectRoleTemplateBindingType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ProjectRoleTemplateBindingClient) List(opts *types.ListOpts) (*ProjectRoleTemplateBindingCollection, error) {
	resp := &ProjectRoleTemplateBindingCollection{}
	err := c.apiClient.Ops.DoList(ProjectRoleTemplateBindingType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ProjectRoleTemplateBindingClient) ListAll(opts *types.ListOpts) (*ProjectRoleTemplateBindingCollection, error) {
	resp := &ProjectRoleTemplateBindingCollection{}
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
