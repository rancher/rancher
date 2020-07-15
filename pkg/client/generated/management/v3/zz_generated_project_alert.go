package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectAlertType                       = "projectAlert"
	ProjectAlertFieldAnnotations           = "annotations"
	ProjectAlertFieldCreated               = "created"
	ProjectAlertFieldCreatorID             = "creatorId"
	ProjectAlertFieldDescription           = "description"
	ProjectAlertFieldDisplayName           = "displayName"
	ProjectAlertFieldInitialWaitSeconds    = "initialWaitSeconds"
	ProjectAlertFieldLabels                = "labels"
	ProjectAlertFieldName                  = "name"
	ProjectAlertFieldNamespaceId           = "namespaceId"
	ProjectAlertFieldOwnerReferences       = "ownerReferences"
	ProjectAlertFieldProjectID             = "projectId"
	ProjectAlertFieldRecipients            = "recipients"
	ProjectAlertFieldRemoved               = "removed"
	ProjectAlertFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ProjectAlertFieldSeverity              = "severity"
	ProjectAlertFieldState                 = "state"
	ProjectAlertFieldStatus                = "status"
	ProjectAlertFieldTargetPod             = "targetPod"
	ProjectAlertFieldTargetWorkload        = "targetWorkload"
	ProjectAlertFieldTransitioning         = "transitioning"
	ProjectAlertFieldTransitioningMessage  = "transitioningMessage"
	ProjectAlertFieldUUID                  = "uuid"
)

type ProjectAlert struct {
	types.Resource
	Annotations           map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created               string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID             string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description           string            `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName           string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	InitialWaitSeconds    int64             `json:"initialWaitSeconds,omitempty" yaml:"initialWaitSeconds,omitempty"`
	Labels                map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                  string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId           string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences       []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID             string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Recipients            []Recipient       `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	Removed               string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	RepeatIntervalSeconds int64             `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	Severity              string            `json:"severity,omitempty" yaml:"severity,omitempty"`
	State                 string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status                *AlertStatus      `json:"status,omitempty" yaml:"status,omitempty"`
	TargetPod             *TargetPod        `json:"targetPod,omitempty" yaml:"targetPod,omitempty"`
	TargetWorkload        *TargetWorkload   `json:"targetWorkload,omitempty" yaml:"targetWorkload,omitempty"`
	Transitioning         string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage  string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                  string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ProjectAlertCollection struct {
	types.Collection
	Data   []ProjectAlert `json:"data,omitempty"`
	client *ProjectAlertClient
}

type ProjectAlertClient struct {
	apiClient *Client
}

type ProjectAlertOperations interface {
	List(opts *types.ListOpts) (*ProjectAlertCollection, error)
	ListAll(opts *types.ListOpts) (*ProjectAlertCollection, error)
	Create(opts *ProjectAlert) (*ProjectAlert, error)
	Update(existing *ProjectAlert, updates interface{}) (*ProjectAlert, error)
	Replace(existing *ProjectAlert) (*ProjectAlert, error)
	ByID(id string) (*ProjectAlert, error)
	Delete(container *ProjectAlert) error
}

func newProjectAlertClient(apiClient *Client) *ProjectAlertClient {
	return &ProjectAlertClient{
		apiClient: apiClient,
	}
}

func (c *ProjectAlertClient) Create(container *ProjectAlert) (*ProjectAlert, error) {
	resp := &ProjectAlert{}
	err := c.apiClient.Ops.DoCreate(ProjectAlertType, container, resp)
	return resp, err
}

func (c *ProjectAlertClient) Update(existing *ProjectAlert, updates interface{}) (*ProjectAlert, error) {
	resp := &ProjectAlert{}
	err := c.apiClient.Ops.DoUpdate(ProjectAlertType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectAlertClient) Replace(obj *ProjectAlert) (*ProjectAlert, error) {
	resp := &ProjectAlert{}
	err := c.apiClient.Ops.DoReplace(ProjectAlertType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ProjectAlertClient) List(opts *types.ListOpts) (*ProjectAlertCollection, error) {
	resp := &ProjectAlertCollection{}
	err := c.apiClient.Ops.DoList(ProjectAlertType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ProjectAlertClient) ListAll(opts *types.ListOpts) (*ProjectAlertCollection, error) {
	resp := &ProjectAlertCollection{}
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

func (cc *ProjectAlertCollection) Next() (*ProjectAlertCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectAlertCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectAlertClient) ByID(id string) (*ProjectAlert, error) {
	resp := &ProjectAlert{}
	err := c.apiClient.Ops.DoByID(ProjectAlertType, id, resp)
	return resp, err
}

func (c *ProjectAlertClient) Delete(container *ProjectAlert) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectAlertType, &container.Resource)
}
