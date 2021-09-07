package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectAlertGroupType                       = "projectAlertGroup"
	ProjectAlertGroupFieldAlertState            = "alertState"
	ProjectAlertGroupFieldAnnotations           = "annotations"
	ProjectAlertGroupFieldCreated               = "created"
	ProjectAlertGroupFieldCreatorID             = "creatorId"
	ProjectAlertGroupFieldDescription           = "description"
	ProjectAlertGroupFieldGroupIntervalSeconds  = "groupIntervalSeconds"
	ProjectAlertGroupFieldGroupWaitSeconds      = "groupWaitSeconds"
	ProjectAlertGroupFieldLabels                = "labels"
	ProjectAlertGroupFieldName                  = "name"
	ProjectAlertGroupFieldNamespaceId           = "namespaceId"
	ProjectAlertGroupFieldOwnerReferences       = "ownerReferences"
	ProjectAlertGroupFieldProjectID             = "projectId"
	ProjectAlertGroupFieldRecipients            = "recipients"
	ProjectAlertGroupFieldRemoved               = "removed"
	ProjectAlertGroupFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ProjectAlertGroupFieldState                 = "state"
	ProjectAlertGroupFieldTransitioning         = "transitioning"
	ProjectAlertGroupFieldTransitioningMessage  = "transitioningMessage"
	ProjectAlertGroupFieldUUID                  = "uuid"
)

type ProjectAlertGroup struct {
	types.Resource
	AlertState            string            `json:"alertState,omitempty" yaml:"alertState,omitempty"`
	Annotations           map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created               string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID             string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description           string            `json:"description,omitempty" yaml:"description,omitempty"`
	GroupIntervalSeconds  int64             `json:"groupIntervalSeconds,omitempty" yaml:"groupIntervalSeconds,omitempty"`
	GroupWaitSeconds      int64             `json:"groupWaitSeconds,omitempty" yaml:"groupWaitSeconds,omitempty"`
	Labels                map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                  string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId           string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences       []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID             string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Recipients            []Recipient       `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	Removed               string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	RepeatIntervalSeconds int64             `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	State                 string            `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning         string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage  string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                  string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ProjectAlertGroupCollection struct {
	types.Collection
	Data   []ProjectAlertGroup `json:"data,omitempty"`
	client *ProjectAlertGroupClient
}

type ProjectAlertGroupClient struct {
	apiClient *Client
}

type ProjectAlertGroupOperations interface {
	List(opts *types.ListOpts) (*ProjectAlertGroupCollection, error)
	ListAll(opts *types.ListOpts) (*ProjectAlertGroupCollection, error)
	Create(opts *ProjectAlertGroup) (*ProjectAlertGroup, error)
	Update(existing *ProjectAlertGroup, updates interface{}) (*ProjectAlertGroup, error)
	Replace(existing *ProjectAlertGroup) (*ProjectAlertGroup, error)
	ByID(id string) (*ProjectAlertGroup, error)
	Delete(container *ProjectAlertGroup) error
}

func newProjectAlertGroupClient(apiClient *Client) *ProjectAlertGroupClient {
	return &ProjectAlertGroupClient{
		apiClient: apiClient,
	}
}

func (c *ProjectAlertGroupClient) Create(container *ProjectAlertGroup) (*ProjectAlertGroup, error) {
	resp := &ProjectAlertGroup{}
	err := c.apiClient.Ops.DoCreate(ProjectAlertGroupType, container, resp)
	return resp, err
}

func (c *ProjectAlertGroupClient) Update(existing *ProjectAlertGroup, updates interface{}) (*ProjectAlertGroup, error) {
	resp := &ProjectAlertGroup{}
	err := c.apiClient.Ops.DoUpdate(ProjectAlertGroupType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectAlertGroupClient) Replace(obj *ProjectAlertGroup) (*ProjectAlertGroup, error) {
	resp := &ProjectAlertGroup{}
	err := c.apiClient.Ops.DoReplace(ProjectAlertGroupType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ProjectAlertGroupClient) List(opts *types.ListOpts) (*ProjectAlertGroupCollection, error) {
	resp := &ProjectAlertGroupCollection{}
	err := c.apiClient.Ops.DoList(ProjectAlertGroupType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ProjectAlertGroupClient) ListAll(opts *types.ListOpts) (*ProjectAlertGroupCollection, error) {
	resp := &ProjectAlertGroupCollection{}
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

func (cc *ProjectAlertGroupCollection) Next() (*ProjectAlertGroupCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectAlertGroupCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectAlertGroupClient) ByID(id string) (*ProjectAlertGroup, error) {
	resp := &ProjectAlertGroup{}
	err := c.apiClient.Ops.DoByID(ProjectAlertGroupType, id, resp)
	return resp, err
}

func (c *ProjectAlertGroupClient) Delete(container *ProjectAlertGroup) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectAlertGroupType, &container.Resource)
}
