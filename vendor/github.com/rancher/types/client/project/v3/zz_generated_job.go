package client

import (
	"github.com/rancher/norman/types"
)

const (
	JobType                       = "job"
	JobFieldActiveDeadlineSeconds = "activeDeadlineSeconds"
	JobFieldAnnotations           = "annotations"
	JobFieldBackoffLimit          = "backoffLimit"
	JobFieldCompletions           = "completions"
	JobFieldCreated               = "created"
	JobFieldCreatorID             = "creatorId"
	JobFieldJobStatus             = "jobStatus"
	JobFieldLabels                = "labels"
	JobFieldManualSelector        = "manualSelector"
	JobFieldName                  = "name"
	JobFieldNamespaceId           = "namespaceId"
	JobFieldOwnerReferences       = "ownerReferences"
	JobFieldParallelism           = "parallelism"
	JobFieldProjectID             = "projectId"
	JobFieldRemoved               = "removed"
	JobFieldSelector              = "selector"
	JobFieldState                 = "state"
	JobFieldTemplate              = "template"
	JobFieldTransitioning         = "transitioning"
	JobFieldTransitioningMessage  = "transitioningMessage"
	JobFieldUuid                  = "uuid"
)

type Job struct {
	types.Resource
	ActiveDeadlineSeconds *int64            `json:"activeDeadlineSeconds,omitempty"`
	Annotations           map[string]string `json:"annotations,omitempty"`
	BackoffLimit          *int64            `json:"backoffLimit,omitempty"`
	Completions           *int64            `json:"completions,omitempty"`
	Created               string            `json:"created,omitempty"`
	CreatorID             string            `json:"creatorId,omitempty"`
	JobStatus             *JobStatus        `json:"jobStatus,omitempty"`
	Labels                map[string]string `json:"labels,omitempty"`
	ManualSelector        *bool             `json:"manualSelector,omitempty"`
	Name                  string            `json:"name,omitempty"`
	NamespaceId           string            `json:"namespaceId,omitempty"`
	OwnerReferences       []OwnerReference  `json:"ownerReferences,omitempty"`
	Parallelism           *int64            `json:"parallelism,omitempty"`
	ProjectID             string            `json:"projectId,omitempty"`
	Removed               string            `json:"removed,omitempty"`
	Selector              *LabelSelector    `json:"selector,omitempty"`
	State                 string            `json:"state,omitempty"`
	Template              *PodTemplateSpec  `json:"template,omitempty"`
	Transitioning         string            `json:"transitioning,omitempty"`
	TransitioningMessage  string            `json:"transitioningMessage,omitempty"`
	Uuid                  string            `json:"uuid,omitempty"`
}
type JobCollection struct {
	types.Collection
	Data   []Job `json:"data,omitempty"`
	client *JobClient
}

type JobClient struct {
	apiClient *Client
}

type JobOperations interface {
	List(opts *types.ListOpts) (*JobCollection, error)
	Create(opts *Job) (*Job, error)
	Update(existing *Job, updates interface{}) (*Job, error)
	ByID(id string) (*Job, error)
	Delete(container *Job) error
}

func newJobClient(apiClient *Client) *JobClient {
	return &JobClient{
		apiClient: apiClient,
	}
}

func (c *JobClient) Create(container *Job) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoCreate(JobType, container, resp)
	return resp, err
}

func (c *JobClient) Update(existing *Job, updates interface{}) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoUpdate(JobType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *JobClient) List(opts *types.ListOpts) (*JobCollection, error) {
	resp := &JobCollection{}
	err := c.apiClient.Ops.DoList(JobType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *JobCollection) Next() (*JobCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &JobCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *JobClient) ByID(id string) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoByID(JobType, id, resp)
	return resp, err
}

func (c *JobClient) Delete(container *Job) error {
	return c.apiClient.Ops.DoResourceDelete(JobType, &container.Resource)
}
