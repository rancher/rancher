package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectNetworkPolicyType                      = "projectNetworkPolicy"
	ProjectNetworkPolicyFieldAnnotations          = "annotations"
	ProjectNetworkPolicyFieldCreated              = "created"
	ProjectNetworkPolicyFieldCreatorID            = "creatorId"
	ProjectNetworkPolicyFieldDescription          = "description"
	ProjectNetworkPolicyFieldLabels               = "labels"
	ProjectNetworkPolicyFieldName                 = "name"
	ProjectNetworkPolicyFieldNamespaceId          = "namespaceId"
	ProjectNetworkPolicyFieldOwnerReferences      = "ownerReferences"
	ProjectNetworkPolicyFieldProjectId            = "projectId"
	ProjectNetworkPolicyFieldRemoved              = "removed"
	ProjectNetworkPolicyFieldState                = "state"
	ProjectNetworkPolicyFieldStatus               = "status"
	ProjectNetworkPolicyFieldTransitioning        = "transitioning"
	ProjectNetworkPolicyFieldTransitioningMessage = "transitioningMessage"
	ProjectNetworkPolicyFieldUuid                 = "uuid"
)

type ProjectNetworkPolicy struct {
	types.Resource
	Annotations          map[string]string           `json:"annotations,omitempty"`
	Created              string                      `json:"created,omitempty"`
	CreatorID            string                      `json:"creatorId,omitempty"`
	Description          string                      `json:"description,omitempty"`
	Labels               map[string]string           `json:"labels,omitempty"`
	Name                 string                      `json:"name,omitempty"`
	NamespaceId          string                      `json:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference            `json:"ownerReferences,omitempty"`
	ProjectId            string                      `json:"projectId,omitempty"`
	Removed              string                      `json:"removed,omitempty"`
	State                string                      `json:"state,omitempty"`
	Status               *ProjectNetworkPolicyStatus `json:"status,omitempty"`
	Transitioning        string                      `json:"transitioning,omitempty"`
	TransitioningMessage string                      `json:"transitioningMessage,omitempty"`
	Uuid                 string                      `json:"uuid,omitempty"`
}
type ProjectNetworkPolicyCollection struct {
	types.Collection
	Data   []ProjectNetworkPolicy `json:"data,omitempty"`
	client *ProjectNetworkPolicyClient
}

type ProjectNetworkPolicyClient struct {
	apiClient *Client
}

type ProjectNetworkPolicyOperations interface {
	List(opts *types.ListOpts) (*ProjectNetworkPolicyCollection, error)
	Create(opts *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error)
	Update(existing *ProjectNetworkPolicy, updates interface{}) (*ProjectNetworkPolicy, error)
	ByID(id string) (*ProjectNetworkPolicy, error)
	Delete(container *ProjectNetworkPolicy) error
}

func newProjectNetworkPolicyClient(apiClient *Client) *ProjectNetworkPolicyClient {
	return &ProjectNetworkPolicyClient{
		apiClient: apiClient,
	}
}

func (c *ProjectNetworkPolicyClient) Create(container *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error) {
	resp := &ProjectNetworkPolicy{}
	err := c.apiClient.Ops.DoCreate(ProjectNetworkPolicyType, container, resp)
	return resp, err
}

func (c *ProjectNetworkPolicyClient) Update(existing *ProjectNetworkPolicy, updates interface{}) (*ProjectNetworkPolicy, error) {
	resp := &ProjectNetworkPolicy{}
	err := c.apiClient.Ops.DoUpdate(ProjectNetworkPolicyType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectNetworkPolicyClient) List(opts *types.ListOpts) (*ProjectNetworkPolicyCollection, error) {
	resp := &ProjectNetworkPolicyCollection{}
	err := c.apiClient.Ops.DoList(ProjectNetworkPolicyType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ProjectNetworkPolicyCollection) Next() (*ProjectNetworkPolicyCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectNetworkPolicyCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectNetworkPolicyClient) ByID(id string) (*ProjectNetworkPolicy, error) {
	resp := &ProjectNetworkPolicy{}
	err := c.apiClient.Ops.DoByID(ProjectNetworkPolicyType, id, resp)
	return resp, err
}

func (c *ProjectNetworkPolicyClient) Delete(container *ProjectNetworkPolicy) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectNetworkPolicyType, &container.Resource)
}
