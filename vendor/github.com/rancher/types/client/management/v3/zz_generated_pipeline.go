package client

import (
	"github.com/rancher/norman/types"
)

const (
	PipelineType                       = "pipeline"
	PipelineFieldAnnotations           = "annotations"
	PipelineFieldCreated               = "created"
	PipelineFieldCreatorID             = "creatorId"
	PipelineFieldLabels                = "labels"
	PipelineFieldLastExecutionID       = "lastExecutionId"
	PipelineFieldLastRunState          = "lastRunState"
	PipelineFieldLastStarted           = "lastStarted"
	PipelineFieldName                  = "name"
	PipelineFieldNamespaceId           = "namespaceId"
	PipelineFieldNextRun               = "nextRun"
	PipelineFieldNextStart             = "nextStart"
	PipelineFieldOwnerReferences       = "ownerReferences"
	PipelineFieldPipelineState         = "pipelineState"
	PipelineFieldProjectId             = "projectId"
	PipelineFieldRemoved               = "removed"
	PipelineFieldStages                = "stages"
	PipelineFieldState                 = "state"
	PipelineFieldToken                 = "token"
	PipelineFieldTransitioning         = "transitioning"
	PipelineFieldTransitioningMessage  = "transitioningMessage"
	PipelineFieldTriggerCronExpression = "triggerCronExpression"
	PipelineFieldTriggerCronTimezone   = "triggerCronTimezone"
	PipelineFieldTriggerWebhook        = "triggerWebhook"
	PipelineFieldUuid                  = "uuid"
	PipelineFieldWebHookID             = "webhookId"
)

type Pipeline struct {
	types.Resource
	Annotations           map[string]string `json:"annotations,omitempty"`
	Created               string            `json:"created,omitempty"`
	CreatorID             string            `json:"creatorId,omitempty"`
	Labels                map[string]string `json:"labels,omitempty"`
	LastExecutionID       string            `json:"lastExecutionId,omitempty"`
	LastRunState          string            `json:"lastRunState,omitempty"`
	LastStarted           string            `json:"lastStarted,omitempty"`
	Name                  string            `json:"name,omitempty"`
	NamespaceId           string            `json:"namespaceId,omitempty"`
	NextRun               *int64            `json:"nextRun,omitempty"`
	NextStart             string            `json:"nextStart,omitempty"`
	OwnerReferences       []OwnerReference  `json:"ownerReferences,omitempty"`
	PipelineState         string            `json:"pipelineState,omitempty"`
	ProjectId             string            `json:"projectId,omitempty"`
	Removed               string            `json:"removed,omitempty"`
	Stages                []Stage           `json:"stages,omitempty"`
	State                 string            `json:"state,omitempty"`
	Token                 string            `json:"token,omitempty"`
	Transitioning         string            `json:"transitioning,omitempty"`
	TransitioningMessage  string            `json:"transitioningMessage,omitempty"`
	TriggerCronExpression string            `json:"triggerCronExpression,omitempty"`
	TriggerCronTimezone   string            `json:"triggerCronTimezone,omitempty"`
	TriggerWebhook        bool              `json:"triggerWebhook,omitempty"`
	Uuid                  string            `json:"uuid,omitempty"`
	WebHookID             string            `json:"webhookId,omitempty"`
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
