package client

import (
	"github.com/rancher/norman/types"
)

const (
	PipelineType                       = "pipeline"
	PipelineFieldAnnotations           = "annotations"
	PipelineFieldCreated               = "created"
	PipelineFieldCreatorID             = "creatorId"
	PipelineFieldDisplayName           = "displayName"
	PipelineFieldLabels                = "labels"
	PipelineFieldName                  = "name"
	PipelineFieldNamespaceId           = "namespaceId"
	PipelineFieldOwnerReferences       = "ownerReferences"
	PipelineFieldProjectId             = "projectId"
	PipelineFieldRemoved               = "removed"
	PipelineFieldStages                = "stages"
	PipelineFieldState                 = "state"
	PipelineFieldStatus                = "status"
	PipelineFieldTransitioning         = "transitioning"
	PipelineFieldTransitioningMessage  = "transitioningMessage"
	PipelineFieldTriggerCronExpression = "triggerCronExpression"
	PipelineFieldTriggerCronTimezone   = "triggerCronTimezone"
	PipelineFieldTriggerWebhook        = "triggerWebhook"
	PipelineFieldUuid                  = "uuid"
)

type Pipeline struct {
	types.Resource
	Annotations           map[string]string `json:"annotations,omitempty"`
	Created               string            `json:"created,omitempty"`
	CreatorID             string            `json:"creatorId,omitempty"`
	DisplayName           string            `json:"displayName,omitempty"`
	Labels                map[string]string `json:"labels,omitempty"`
	Name                  string            `json:"name,omitempty"`
	NamespaceId           string            `json:"namespaceId,omitempty"`
	OwnerReferences       []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectId             string            `json:"projectId,omitempty"`
	Removed               string            `json:"removed,omitempty"`
	Stages                []Stage           `json:"stages,omitempty"`
	State                 string            `json:"state,omitempty"`
	Status                *PipelineStatus   `json:"status,omitempty"`
	Transitioning         string            `json:"transitioning,omitempty"`
	TransitioningMessage  string            `json:"transitioningMessage,omitempty"`
	TriggerCronExpression string            `json:"triggerCronExpression,omitempty"`
	TriggerCronTimezone   string            `json:"triggerCronTimezone,omitempty"`
	TriggerWebhook        bool              `json:"triggerWebhook,omitempty"`
	Uuid                  string            `json:"uuid,omitempty"`
}
type PipelineCollection struct {
	types.Collection
	Data   []Pipeline `json:"data,omitempty"`
	client *PipelineClient
}

type PipelineClient struct {
	apiClient *Client
}

type PipelineOperations interface {
	List(opts *types.ListOpts) (*PipelineCollection, error)
	Create(opts *Pipeline) (*Pipeline, error)
	Update(existing *Pipeline, updates interface{}) (*Pipeline, error)
	ByID(id string) (*Pipeline, error)
	Delete(container *Pipeline) error
}

func newPipelineClient(apiClient *Client) *PipelineClient {
	return &PipelineClient{
		apiClient: apiClient,
	}
}

func (c *PipelineClient) Create(container *Pipeline) (*Pipeline, error) {
	resp := &Pipeline{}
	err := c.apiClient.Ops.DoCreate(PipelineType, container, resp)
	return resp, err
}

func (c *PipelineClient) Update(existing *Pipeline, updates interface{}) (*Pipeline, error) {
	resp := &Pipeline{}
	err := c.apiClient.Ops.DoUpdate(PipelineType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PipelineClient) List(opts *types.ListOpts) (*PipelineCollection, error) {
	resp := &PipelineCollection{}
	err := c.apiClient.Ops.DoList(PipelineType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *PipelineCollection) Next() (*PipelineCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PipelineCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PipelineClient) ByID(id string) (*Pipeline, error) {
	resp := &Pipeline{}
	err := c.apiClient.Ops.DoByID(PipelineType, id, resp)
	return resp, err
}

func (c *PipelineClient) Delete(container *Pipeline) error {
	return c.apiClient.Ops.DoResourceDelete(PipelineType, &container.Resource)
}
