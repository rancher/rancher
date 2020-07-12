package client

import (
	"github.com/rancher/norman/types"
)

const (
	MultiClusterAppType                      = "multiClusterApp"
	MultiClusterAppFieldAnnotations          = "annotations"
	MultiClusterAppFieldAnswers              = "answers"
	MultiClusterAppFieldCreated              = "created"
	MultiClusterAppFieldCreatorID            = "creatorId"
	MultiClusterAppFieldLabels               = "labels"
	MultiClusterAppFieldMembers              = "members"
	MultiClusterAppFieldName                 = "name"
	MultiClusterAppFieldOwnerReferences      = "ownerReferences"
	MultiClusterAppFieldRemoved              = "removed"
	MultiClusterAppFieldRevisionHistoryLimit = "revisionHistoryLimit"
	MultiClusterAppFieldRoles                = "roles"
	MultiClusterAppFieldState                = "state"
	MultiClusterAppFieldStatus               = "status"
	MultiClusterAppFieldTargets              = "targets"
	MultiClusterAppFieldTemplateVersionID    = "templateVersionId"
	MultiClusterAppFieldTimeout              = "timeout"
	MultiClusterAppFieldTransitioning        = "transitioning"
	MultiClusterAppFieldTransitioningMessage = "transitioningMessage"
	MultiClusterAppFieldUUID                 = "uuid"
	MultiClusterAppFieldUpgradeStrategy      = "upgradeStrategy"
	MultiClusterAppFieldWait                 = "wait"
)

type MultiClusterApp struct {
	types.Resource
	Annotations          map[string]string      `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Answers              []Answer               `json:"answers,omitempty" yaml:"answers,omitempty"`
	Created              string                 `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string                 `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels               map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Members              []Member               `json:"members,omitempty" yaml:"members,omitempty"`
	Name                 string                 `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference       `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string                 `json:"removed,omitempty" yaml:"removed,omitempty"`
	RevisionHistoryLimit int64                  `json:"revisionHistoryLimit,omitempty" yaml:"revisionHistoryLimit,omitempty"`
	Roles                []string               `json:"roles,omitempty" yaml:"roles,omitempty"`
	State                string                 `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *MultiClusterAppStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Targets              []Target               `json:"targets,omitempty" yaml:"targets,omitempty"`
	TemplateVersionID    string                 `json:"templateVersionId,omitempty" yaml:"templateVersionId,omitempty"`
	Timeout              int64                  `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Transitioning        string                 `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string                 `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string                 `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UpgradeStrategy      *UpgradeStrategy       `json:"upgradeStrategy,omitempty" yaml:"upgradeStrategy,omitempty"`
	Wait                 bool                   `json:"wait,omitempty" yaml:"wait,omitempty"`
}

type MultiClusterAppCollection struct {
	types.Collection
	Data   []MultiClusterApp `json:"data,omitempty"`
	client *MultiClusterAppClient
}

type MultiClusterAppClient struct {
	apiClient *Client
}

type MultiClusterAppOperations interface {
	List(opts *types.ListOpts) (*MultiClusterAppCollection, error)
	ListAll(opts *types.ListOpts) (*MultiClusterAppCollection, error)
	Create(opts *MultiClusterApp) (*MultiClusterApp, error)
	Update(existing *MultiClusterApp, updates interface{}) (*MultiClusterApp, error)
	Replace(existing *MultiClusterApp) (*MultiClusterApp, error)
	ByID(id string) (*MultiClusterApp, error)
	Delete(container *MultiClusterApp) error

	ActionAddProjects(resource *MultiClusterApp, input *UpdateMultiClusterAppTargetsInput) error

	ActionRemoveProjects(resource *MultiClusterApp, input *UpdateMultiClusterAppTargetsInput) error

	ActionRollback(resource *MultiClusterApp, input *MultiClusterAppRollbackInput) error
}

func newMultiClusterAppClient(apiClient *Client) *MultiClusterAppClient {
	return &MultiClusterAppClient{
		apiClient: apiClient,
	}
}

func (c *MultiClusterAppClient) Create(container *MultiClusterApp) (*MultiClusterApp, error) {
	resp := &MultiClusterApp{}
	err := c.apiClient.Ops.DoCreate(MultiClusterAppType, container, resp)
	return resp, err
}

func (c *MultiClusterAppClient) Update(existing *MultiClusterApp, updates interface{}) (*MultiClusterApp, error) {
	resp := &MultiClusterApp{}
	err := c.apiClient.Ops.DoUpdate(MultiClusterAppType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *MultiClusterAppClient) Replace(obj *MultiClusterApp) (*MultiClusterApp, error) {
	resp := &MultiClusterApp{}
	err := c.apiClient.Ops.DoReplace(MultiClusterAppType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *MultiClusterAppClient) List(opts *types.ListOpts) (*MultiClusterAppCollection, error) {
	resp := &MultiClusterAppCollection{}
	err := c.apiClient.Ops.DoList(MultiClusterAppType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *MultiClusterAppClient) ListAll(opts *types.ListOpts) (*MultiClusterAppCollection, error) {
	resp := &MultiClusterAppCollection{}
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

func (cc *MultiClusterAppCollection) Next() (*MultiClusterAppCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &MultiClusterAppCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *MultiClusterAppClient) ByID(id string) (*MultiClusterApp, error) {
	resp := &MultiClusterApp{}
	err := c.apiClient.Ops.DoByID(MultiClusterAppType, id, resp)
	return resp, err
}

func (c *MultiClusterAppClient) Delete(container *MultiClusterApp) error {
	return c.apiClient.Ops.DoResourceDelete(MultiClusterAppType, &container.Resource)
}

func (c *MultiClusterAppClient) ActionAddProjects(resource *MultiClusterApp, input *UpdateMultiClusterAppTargetsInput) error {
	err := c.apiClient.Ops.DoAction(MultiClusterAppType, "addProjects", &resource.Resource, input, nil)
	return err
}

func (c *MultiClusterAppClient) ActionRemoveProjects(resource *MultiClusterApp, input *UpdateMultiClusterAppTargetsInput) error {
	err := c.apiClient.Ops.DoAction(MultiClusterAppType, "removeProjects", &resource.Resource, input, nil)
	return err
}

func (c *MultiClusterAppClient) ActionRollback(resource *MultiClusterApp, input *MultiClusterAppRollbackInput) error {
	err := c.apiClient.Ops.DoAction(MultiClusterAppType, "rollback", &resource.Resource, input, nil)
	return err
}
