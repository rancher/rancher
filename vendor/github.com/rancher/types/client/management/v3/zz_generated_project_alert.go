package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectAlertType                       = "projectAlert"
	ProjectAlertFieldAlertState            = "alertState"
	ProjectAlertFieldAnnotations           = "annotations"
	ProjectAlertFieldCreated               = "created"
	ProjectAlertFieldCreatorID             = "creatorId"
	ProjectAlertFieldDescription           = "description"
	ProjectAlertFieldInitialWaitSeconds    = "initialWaitSeconds"
	ProjectAlertFieldLabels                = "labels"
	ProjectAlertFieldName                  = "name"
	ProjectAlertFieldNamespaceId           = "namespaceId"
	ProjectAlertFieldOwnerReferences       = "ownerReferences"
	ProjectAlertFieldProjectId             = "projectId"
	ProjectAlertFieldRecipients            = "recipients"
	ProjectAlertFieldRemoved               = "removed"
	ProjectAlertFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ProjectAlertFieldSeverity              = "severity"
	ProjectAlertFieldState                 = "state"
	ProjectAlertFieldTargetPod             = "targetPod"
	ProjectAlertFieldTargetWorkload        = "targetWorkload"
	ProjectAlertFieldTransitioning         = "transitioning"
	ProjectAlertFieldTransitioningMessage  = "transitioningMessage"
	ProjectAlertFieldUuid                  = "uuid"
)

type ProjectAlert struct {
	types.Resource
	AlertState            string            `json:"alertState,omitempty"`
	Annotations           map[string]string `json:"annotations,omitempty"`
	Created               string            `json:"created,omitempty"`
	CreatorID             string            `json:"creatorId,omitempty"`
	Description           string            `json:"description,omitempty"`
	InitialWaitSeconds    *int64            `json:"initialWaitSeconds,omitempty"`
	Labels                map[string]string `json:"labels,omitempty"`
	Name                  string            `json:"name,omitempty"`
	NamespaceId           string            `json:"namespaceId,omitempty"`
	OwnerReferences       []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectId             string            `json:"projectId,omitempty"`
	Recipients            []Recipient       `json:"recipients,omitempty"`
	Removed               string            `json:"removed,omitempty"`
	RepeatIntervalSeconds *int64            `json:"repeatIntervalSeconds,omitempty"`
	Severity              string            `json:"severity,omitempty"`
	State                 string            `json:"state,omitempty"`
	TargetPod             *TargetPod        `json:"targetPod,omitempty"`
	TargetWorkload        *TargetWorkload   `json:"targetWorkload,omitempty"`
	Transitioning         string            `json:"transitioning,omitempty"`
	TransitioningMessage  string            `json:"transitioningMessage,omitempty"`
	Uuid                  string            `json:"uuid,omitempty"`
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
	Create(opts *ProjectAlert) (*ProjectAlert, error)
	Update(existing *ProjectAlert, updates interface{}) (*ProjectAlert, error)
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

func (c *ProjectAlertClient) List(opts *types.ListOpts) (*ProjectAlertCollection, error) {
	resp := &ProjectAlertCollection{}
	err := c.apiClient.Ops.DoList(ProjectAlertType, opts, resp)
	resp.client = c
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
