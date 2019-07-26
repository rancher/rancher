package client

import (
	"github.com/rancher/norman/types"
)

const (
	AppType                      = "app"
	AppFieldAnnotations          = "annotations"
	AppFieldAnswers              = "answers"
	AppFieldAppRevisionID        = "appRevisionId"
	AppFieldAppliedFiles         = "appliedFiles"
	AppFieldConditions           = "conditions"
	AppFieldCreated              = "created"
	AppFieldCreatorID            = "creatorId"
	AppFieldDescription          = "description"
	AppFieldExternalID           = "externalId"
	AppFieldFiles                = "files"
	AppFieldLabels               = "labels"
	AppFieldLastAppliedTemplates = "lastAppliedTemplate"
	AppFieldMultiClusterAppID    = "multiClusterAppId"
	AppFieldName                 = "name"
	AppFieldNamespaceId          = "namespaceId"
	AppFieldNotes                = "notes"
	AppFieldOwnerReferences      = "ownerReferences"
	AppFieldProjectID            = "projectId"
	AppFieldPrune                = "prune"
	AppFieldRemoved              = "removed"
	AppFieldState                = "state"
	AppFieldTargetNamespace      = "targetNamespace"
	AppFieldTimeout              = "timeout"
	AppFieldTransitioning        = "transitioning"
	AppFieldTransitioningMessage = "transitioningMessage"
	AppFieldUUID                 = "uuid"
	AppFieldValuesYaml           = "valuesYaml"
	AppFieldWait                 = "wait"
)

type App struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Answers              map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AppRevisionID        string            `json:"appRevisionId,omitempty" yaml:"appRevisionId,omitempty"`
	AppliedFiles         map[string]string `json:"appliedFiles,omitempty" yaml:"appliedFiles,omitempty"`
	Conditions           []AppCondition    `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description          string            `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalID           string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files                map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LastAppliedTemplates string            `json:"lastAppliedTemplate,omitempty" yaml:"lastAppliedTemplate,omitempty"`
	MultiClusterAppID    string            `json:"multiClusterAppId,omitempty" yaml:"multiClusterAppId,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	Notes                string            `json:"notes,omitempty" yaml:"notes,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID            string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Prune                bool              `json:"prune,omitempty" yaml:"prune,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	TargetNamespace      string            `json:"targetNamespace,omitempty" yaml:"targetNamespace,omitempty"`
	Timeout              int64             `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	ValuesYaml           string            `json:"valuesYaml,omitempty" yaml:"valuesYaml,omitempty"`
	Wait                 bool              `json:"wait,omitempty" yaml:"wait,omitempty"`
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
	Replace(existing *App) (*App, error)
	ByID(id string) (*App, error)
	Delete(container *App) error

	ActionRollback(resource *App, input *RollbackRevision) error

	ActionUpgrade(resource *App, input *AppUpgradeConfig) error
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

func (c *AppClient) Replace(obj *App) (*App, error) {
	resp := &App{}
	err := c.apiClient.Ops.DoReplace(AppType, &obj.Resource, obj, resp)
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

func (c *AppClient) ActionRollback(resource *App, input *RollbackRevision) error {
	err := c.apiClient.Ops.DoAction(AppType, "rollback", &resource.Resource, input, nil)
	return err
}

func (c *AppClient) ActionUpgrade(resource *App, input *AppUpgradeConfig) error {
	err := c.apiClient.Ops.DoAction(AppType, "upgrade", &resource.Resource, input, nil)
	return err
}
