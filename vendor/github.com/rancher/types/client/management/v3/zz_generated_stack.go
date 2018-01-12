package client

import (
	"github.com/rancher/norman/types"
)

const (
	StackType                      = "stack"
	StackFieldAnnotations          = "annotations"
	StackFieldAnswers              = "answers"
	StackFieldCreated              = "created"
	StackFieldCreatorID            = "creatorId"
	StackFieldDescription          = "description"
	StackFieldExternalID           = "externalId"
	StackFieldGroups               = "groups"
	StackFieldInstallNamespace     = "installNamespace"
	StackFieldLabels               = "labels"
	StackFieldName                 = "name"
	StackFieldNamespaceId          = "namespaceId"
	StackFieldOwnerReferences      = "ownerReferences"
	StackFieldProjectId            = "projectId"
	StackFieldPrune                = "prune"
	StackFieldRemoved              = "removed"
	StackFieldState                = "state"
	StackFieldStatus               = "status"
	StackFieldTag                  = "tag"
	StackFieldTemplates            = "templates"
	StackFieldTransitioning        = "transitioning"
	StackFieldTransitioningMessage = "transitioningMessage"
	StackFieldUser                 = "user"
	StackFieldUuid                 = "uuid"
)

type Stack struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty"`
	Answers              map[string]string `json:"answers,omitempty"`
	Created              string            `json:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty"`
	Description          string            `json:"description,omitempty"`
	ExternalID           string            `json:"externalId,omitempty"`
	Groups               []string          `json:"groups,omitempty"`
	InstallNamespace     string            `json:"installNamespace,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Name                 string            `json:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectId            string            `json:"projectId,omitempty"`
	Prune                *bool             `json:"prune,omitempty"`
	Removed              string            `json:"removed,omitempty"`
	State                string            `json:"state,omitempty"`
	Status               *StackStatus      `json:"status,omitempty"`
	Tag                  map[string]string `json:"tag,omitempty"`
	Templates            map[string]string `json:"templates,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty"`
	User                 string            `json:"user,omitempty"`
	Uuid                 string            `json:"uuid,omitempty"`
}
type StackCollection struct {
	types.Collection
	Data   []Stack `json:"data,omitempty"`
	client *StackClient
}

type StackClient struct {
	apiClient *Client
}

type StackOperations interface {
	List(opts *types.ListOpts) (*StackCollection, error)
	Create(opts *Stack) (*Stack, error)
	Update(existing *Stack, updates interface{}) (*Stack, error)
	ByID(id string) (*Stack, error)
	Delete(container *Stack) error
}

func newStackClient(apiClient *Client) *StackClient {
	return &StackClient{
		apiClient: apiClient,
	}
}

func (c *StackClient) Create(container *Stack) (*Stack, error) {
	resp := &Stack{}
	err := c.apiClient.Ops.DoCreate(StackType, container, resp)
	return resp, err
}

func (c *StackClient) Update(existing *Stack, updates interface{}) (*Stack, error) {
	resp := &Stack{}
	err := c.apiClient.Ops.DoUpdate(StackType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *StackClient) List(opts *types.ListOpts) (*StackCollection, error) {
	resp := &StackCollection{}
	err := c.apiClient.Ops.DoList(StackType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *StackCollection) Next() (*StackCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &StackCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *StackClient) ByID(id string) (*Stack, error) {
	resp := &Stack{}
	err := c.apiClient.Ops.DoByID(StackType, id, resp)
	return resp, err
}

func (c *StackClient) Delete(container *Stack) error {
	return c.apiClient.Ops.DoResourceDelete(StackType, &container.Resource)
}
