package client

import (
	"github.com/rancher/norman/types"
)

const (
	PipelineExecutionLogType                     = "pipelineExecutionLog"
	PipelineExecutionLogFieldAnnotations         = "annotations"
	PipelineExecutionLogFieldCreated             = "created"
	PipelineExecutionLogFieldCreatorID           = "creatorId"
	PipelineExecutionLogFieldLabels              = "labels"
	PipelineExecutionLogFieldLine                = "line"
	PipelineExecutionLogFieldMessage             = "message"
	PipelineExecutionLogFieldName                = "name"
	PipelineExecutionLogFieldNamespaceId         = "namespaceId"
	PipelineExecutionLogFieldOwnerReferences     = "ownerReferences"
	PipelineExecutionLogFieldPipelineExecutionId = "pipelineExecutionId"
	PipelineExecutionLogFieldProjectId           = "projectId"
	PipelineExecutionLogFieldRemoved             = "removed"
	PipelineExecutionLogFieldStage               = "stage"
	PipelineExecutionLogFieldStep                = "step"
	PipelineExecutionLogFieldUuid                = "uuid"
)

type PipelineExecutionLog struct {
	types.Resource
	Annotations         map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created             string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels              map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Line                *int64            `json:"line,omitempty" yaml:"line,omitempty"`
	Message             string            `json:"message,omitempty" yaml:"message,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId         string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PipelineExecutionId string            `json:"pipelineExecutionId,omitempty" yaml:"pipelineExecutionId,omitempty"`
	ProjectId           string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed             string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Stage               *int64            `json:"stage,omitempty" yaml:"stage,omitempty"`
	Step                *int64            `json:"step,omitempty" yaml:"step,omitempty"`
	Uuid                string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type PipelineExecutionLogCollection struct {
	types.Collection
	Data   []PipelineExecutionLog `json:"data,omitempty"`
	client *PipelineExecutionLogClient
}

type PipelineExecutionLogClient struct {
	apiClient *Client
}

type PipelineExecutionLogOperations interface {
	List(opts *types.ListOpts) (*PipelineExecutionLogCollection, error)
	Create(opts *PipelineExecutionLog) (*PipelineExecutionLog, error)
	Update(existing *PipelineExecutionLog, updates interface{}) (*PipelineExecutionLog, error)
	ByID(id string) (*PipelineExecutionLog, error)
	Delete(container *PipelineExecutionLog) error
}

func newPipelineExecutionLogClient(apiClient *Client) *PipelineExecutionLogClient {
	return &PipelineExecutionLogClient{
		apiClient: apiClient,
	}
}

func (c *PipelineExecutionLogClient) Create(container *PipelineExecutionLog) (*PipelineExecutionLog, error) {
	resp := &PipelineExecutionLog{}
	err := c.apiClient.Ops.DoCreate(PipelineExecutionLogType, container, resp)
	return resp, err
}

func (c *PipelineExecutionLogClient) Update(existing *PipelineExecutionLog, updates interface{}) (*PipelineExecutionLog, error) {
	resp := &PipelineExecutionLog{}
	err := c.apiClient.Ops.DoUpdate(PipelineExecutionLogType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PipelineExecutionLogClient) List(opts *types.ListOpts) (*PipelineExecutionLogCollection, error) {
	resp := &PipelineExecutionLogCollection{}
	err := c.apiClient.Ops.DoList(PipelineExecutionLogType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *PipelineExecutionLogCollection) Next() (*PipelineExecutionLogCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PipelineExecutionLogCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PipelineExecutionLogClient) ByID(id string) (*PipelineExecutionLog, error) {
	resp := &PipelineExecutionLog{}
	err := c.apiClient.Ops.DoByID(PipelineExecutionLogType, id, resp)
	return resp, err
}

func (c *PipelineExecutionLogClient) Delete(container *PipelineExecutionLog) error {
	return c.apiClient.Ops.DoResourceDelete(PipelineExecutionLogType, &container.Resource)
}
