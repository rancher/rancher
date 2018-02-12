package client

import (
	"github.com/rancher/norman/types"
)

const (
	CronJobType                      = "cronJob"
	CronJobFieldAnnotations          = "annotations"
	CronJobFieldCreated              = "created"
	CronJobFieldCreatorID            = "creatorId"
	CronJobFieldCronJob              = "cronJob"
	CronJobFieldCronJobStatus        = "cronJobStatus"
	CronJobFieldLabels               = "labels"
	CronJobFieldName                 = "name"
	CronJobFieldNamespaceId          = "namespaceId"
	CronJobFieldOwnerReferences      = "ownerReferences"
	CronJobFieldProjectID            = "projectId"
	CronJobFieldRemoved              = "removed"
	CronJobFieldState                = "state"
	CronJobFieldTransitioning        = "transitioning"
	CronJobFieldTransitioningMessage = "transitioningMessage"
	CronJobFieldUuid                 = "uuid"
)

type CronJob struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty"`
	Created              string            `json:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty"`
	CronJob              *CronJobConfig    `json:"cronJob,omitempty"`
	CronJobStatus        *CronJobStatus    `json:"cronJobStatus,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Name                 string            `json:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectID            string            `json:"projectId,omitempty"`
	Removed              string            `json:"removed,omitempty"`
	State                string            `json:"state,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty"`
	Uuid                 string            `json:"uuid,omitempty"`
}
type CronJobCollection struct {
	types.Collection
	Data   []CronJob `json:"data,omitempty"`
	client *CronJobClient
}

type CronJobClient struct {
	apiClient *Client
}

type CronJobOperations interface {
	List(opts *types.ListOpts) (*CronJobCollection, error)
	Create(opts *CronJob) (*CronJob, error)
	Update(existing *CronJob, updates interface{}) (*CronJob, error)
	ByID(id string) (*CronJob, error)
	Delete(container *CronJob) error
}

func newCronJobClient(apiClient *Client) *CronJobClient {
	return &CronJobClient{
		apiClient: apiClient,
	}
}

func (c *CronJobClient) Create(container *CronJob) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoCreate(CronJobType, container, resp)
	return resp, err
}

func (c *CronJobClient) Update(existing *CronJob, updates interface{}) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoUpdate(CronJobType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CronJobClient) List(opts *types.ListOpts) (*CronJobCollection, error) {
	resp := &CronJobCollection{}
	err := c.apiClient.Ops.DoList(CronJobType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *CronJobCollection) Next() (*CronJobCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CronJobCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CronJobClient) ByID(id string) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoByID(CronJobType, id, resp)
	return resp, err
}

func (c *CronJobClient) Delete(container *CronJob) error {
	return c.apiClient.Ops.DoResourceDelete(CronJobType, &container.Resource)
}
