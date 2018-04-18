package client

import (
	"github.com/rancher/norman/types"
)

const (
	PipelineExecutionType                      = "pipelineExecution"
	PipelineExecutionFieldAnnotations          = "annotations"
	PipelineExecutionFieldCommit               = "commit"
	PipelineExecutionFieldConditions           = "conditions"
	PipelineExecutionFieldCreated              = "created"
	PipelineExecutionFieldCreatorID            = "creatorId"
	PipelineExecutionFieldEnded                = "ended"
	PipelineExecutionFieldEnvVars              = "envVars"
	PipelineExecutionFieldExecutionState       = "executionState"
	PipelineExecutionFieldLabels               = "labels"
	PipelineExecutionFieldName                 = "name"
	PipelineExecutionFieldNamespaceId          = "namespaceId"
	PipelineExecutionFieldOwnerReferences      = "ownerReferences"
	PipelineExecutionFieldPipeline             = "pipeline"
	PipelineExecutionFieldPipelineId           = "pipelineId"
	PipelineExecutionFieldProjectId            = "projectId"
	PipelineExecutionFieldRemoved              = "removed"
	PipelineExecutionFieldRun                  = "run"
	PipelineExecutionFieldStages               = "stages"
	PipelineExecutionFieldStarted              = "started"
	PipelineExecutionFieldState                = "state"
	PipelineExecutionFieldTransitioning        = "transitioning"
	PipelineExecutionFieldTransitioningMessage = "transitioningMessage"
	PipelineExecutionFieldTriggerUserId        = "triggerUserId"
	PipelineExecutionFieldTriggeredBy          = "triggeredBy"
	PipelineExecutionFieldUuid                 = "uuid"
)

type PipelineExecution struct {
	types.Resource
	Annotations          map[string]string   `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Commit               string              `json:"commit,omitempty" yaml:"commit,omitempty"`
	Conditions           []PipelineCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created              string              `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string              `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Ended                string              `json:"ended,omitempty" yaml:"ended,omitempty"`
	EnvVars              map[string]string   `json:"envVars,omitempty" yaml:"envVars,omitempty"`
	ExecutionState       string              `json:"executionState,omitempty" yaml:"executionState,omitempty"`
	Labels               map[string]string   `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string              `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string              `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference    `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Pipeline             *Pipeline           `json:"pipeline,omitempty" yaml:"pipeline,omitempty"`
	PipelineId           string              `json:"pipelineId,omitempty" yaml:"pipelineId,omitempty"`
	ProjectId            string              `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed              string              `json:"removed,omitempty" yaml:"removed,omitempty"`
	Run                  int64               `json:"run,omitempty" yaml:"run,omitempty"`
	Stages               []StageStatus       `json:"stages,omitempty" yaml:"stages,omitempty"`
	Started              string              `json:"started,omitempty" yaml:"started,omitempty"`
	State                string              `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning        string              `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string              `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	TriggerUserId        string              `json:"triggerUserId,omitempty" yaml:"triggerUserId,omitempty"`
	TriggeredBy          string              `json:"triggeredBy,omitempty" yaml:"triggeredBy,omitempty"`
	Uuid                 string              `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type PipelineExecutionCollection struct {
	types.Collection
	Data   []PipelineExecution `json:"data,omitempty"`
	client *PipelineExecutionClient
}

type PipelineExecutionClient struct {
	apiClient *Client
}

type PipelineExecutionOperations interface {
	List(opts *types.ListOpts) (*PipelineExecutionCollection, error)
	Create(opts *PipelineExecution) (*PipelineExecution, error)
	Update(existing *PipelineExecution, updates interface{}) (*PipelineExecution, error)
	ByID(id string) (*PipelineExecution, error)
	Delete(container *PipelineExecution) error
}

func newPipelineExecutionClient(apiClient *Client) *PipelineExecutionClient {
	return &PipelineExecutionClient{
		apiClient: apiClient,
	}
}

func (c *PipelineExecutionClient) Create(container *PipelineExecution) (*PipelineExecution, error) {
	resp := &PipelineExecution{}
	err := c.apiClient.Ops.DoCreate(PipelineExecutionType, container, resp)
	return resp, err
}

func (c *PipelineExecutionClient) Update(existing *PipelineExecution, updates interface{}) (*PipelineExecution, error) {
	resp := &PipelineExecution{}
	err := c.apiClient.Ops.DoUpdate(PipelineExecutionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PipelineExecutionClient) List(opts *types.ListOpts) (*PipelineExecutionCollection, error) {
	resp := &PipelineExecutionCollection{}
	err := c.apiClient.Ops.DoList(PipelineExecutionType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *PipelineExecutionCollection) Next() (*PipelineExecutionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PipelineExecutionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PipelineExecutionClient) ByID(id string) (*PipelineExecution, error) {
	resp := &PipelineExecution{}
	err := c.apiClient.Ops.DoByID(PipelineExecutionType, id, resp)
	return resp, err
}

func (c *PipelineExecutionClient) Delete(container *PipelineExecution) error {
	return c.apiClient.Ops.DoResourceDelete(PipelineExecutionType, &container.Resource)
}
