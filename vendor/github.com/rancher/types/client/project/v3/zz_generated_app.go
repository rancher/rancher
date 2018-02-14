package client

import (
	"github.com/rancher/norman/types"
)

const (
	AppType                      = "app"
	AppFieldAnnotations          = "annotations"
	AppFieldAnswers              = "answers"
	AppFieldCreated              = "created"
	AppFieldCreatorID            = "creatorId"
	AppFieldDescription          = "description"
	AppFieldExternalID           = "externalId"
	AppFieldGroups               = "groups"
	AppFieldInstallNamespace     = "installNamespace"
	AppFieldLabels               = "labels"
	AppFieldName                 = "name"
	AppFieldNamespaceId          = "namespaceId"
	AppFieldOwnerReferences      = "ownerReferences"
	AppFieldProjectId            = "projectId"
	AppFieldPrune                = "prune"
	AppFieldRemoved              = "removed"
	AppFieldState                = "state"
	AppFieldStatus               = "status"
	AppFieldTag                  = "tag"
	AppFieldTemplates            = "templates"
	AppFieldTransitioning        = "transitioning"
	AppFieldTransitioningMessage = "transitioningMessage"
	AppFieldUser                 = "user"
	AppFieldUuid                 = "uuid"
)

type App struct {
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
	Prune                bool              `json:"prune,omitempty"`
	Removed              string            `json:"removed,omitempty"`
	State                string            `json:"state,omitempty"`
	Status               *AppStatus        `json:"status,omitempty"`
	Tag                  map[string]string `json:"tag,omitempty"`
	Templates            map[string]string `json:"templates,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty"`
	User                 string            `json:"user,omitempty"`
	Uuid                 string            `json:"uuid,omitempty"`
}
type AppCollection struct {
	types.Collection
	Data   []App `json:"data,omitempty"`
	client *AppClient
}

type AppClient struct {
	apiClient *Client
}

type AppOperations interface {
	List(opts *types.ListOpts) (*AppCollection, error)
	Create(opts *App) (*App, error)
	Update(existing *App, updates interface{}) (*App, error)
	ByID(id string) (*App, error)
	Delete(container *App) error
}

func newAppClient(apiClient *Client) *AppClient {
	return &AppClient{
		apiClient: apiClient,
	}
}

func (c *AppClient) Create(container *App) (*App, error) {
	resp := &App{}
	err := c.apiClient.Ops.DoCreate(AppType, container, resp)
	return resp, err
}

func (c *AppClient) Update(existing *App, updates interface{}) (*App, error) {
	resp := &App{}
	err := c.apiClient.Ops.DoUpdate(AppType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *AppClient) List(opts *types.ListOpts) (*AppCollection, error) {
	resp := &AppCollection{}
	err := c.apiClient.Ops.DoList(AppType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *AppCollection) Next() (*AppCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &AppCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *AppClient) ByID(id string) (*App, error) {
	resp := &App{}
	err := c.apiClient.Ops.DoByID(AppType, id, resp)
	return resp, err
}

func (c *AppClient) Delete(container *App) error {
	return c.apiClient.Ops.DoResourceDelete(AppType, &container.Resource)
}
