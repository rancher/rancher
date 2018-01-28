package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectLoggingType                      = "projectLogging"
	ProjectLoggingFieldAnnotations          = "annotations"
	ProjectLoggingFieldCreated              = "created"
	ProjectLoggingFieldCreatorID            = "creatorId"
	ProjectLoggingFieldDisplayName          = "displayName"
	ProjectLoggingFieldElasticsearchConfig  = "elasticsearchConfig"
	ProjectLoggingFieldKafkaConfig          = "kafkaConfig"
	ProjectLoggingFieldLabels               = "labels"
	ProjectLoggingFieldName                 = "name"
	ProjectLoggingFieldNamespaceId          = "namespaceId"
	ProjectLoggingFieldOutputFlushInterval  = "outputFlushInterval"
	ProjectLoggingFieldOutputTags           = "outputTags"
	ProjectLoggingFieldOwnerReferences      = "ownerReferences"
	ProjectLoggingFieldProjectId            = "projectId"
	ProjectLoggingFieldRemoved              = "removed"
	ProjectLoggingFieldSplunkConfig         = "splunkConfig"
	ProjectLoggingFieldState                = "state"
	ProjectLoggingFieldStatus               = "status"
	ProjectLoggingFieldSyslogConfig         = "syslogConfig"
	ProjectLoggingFieldTransitioning        = "transitioning"
	ProjectLoggingFieldTransitioningMessage = "transitioningMessage"
	ProjectLoggingFieldUuid                 = "uuid"
)

type ProjectLogging struct {
	types.Resource
	Annotations          map[string]string    `json:"annotations,omitempty"`
	Created              string               `json:"created,omitempty"`
	CreatorID            string               `json:"creatorId,omitempty"`
	DisplayName          string               `json:"displayName,omitempty"`
	ElasticsearchConfig  *ElasticsearchConfig `json:"elasticsearchConfig,omitempty"`
	KafkaConfig          *KafkaConfig         `json:"kafkaConfig,omitempty"`
	Labels               map[string]string    `json:"labels,omitempty"`
	Name                 string               `json:"name,omitempty"`
	NamespaceId          string               `json:"namespaceId,omitempty"`
	OutputFlushInterval  *int64               `json:"outputFlushInterval,omitempty"`
	OutputTags           map[string]string    `json:"outputTags,omitempty"`
	OwnerReferences      []OwnerReference     `json:"ownerReferences,omitempty"`
	ProjectId            string               `json:"projectId,omitempty"`
	Removed              string               `json:"removed,omitempty"`
	SplunkConfig         *SplunkConfig        `json:"splunkConfig,omitempty"`
	State                string               `json:"state,omitempty"`
	Status               *LoggingStatus       `json:"status,omitempty"`
	SyslogConfig         *SyslogConfig        `json:"syslogConfig,omitempty"`
	Transitioning        string               `json:"transitioning,omitempty"`
	TransitioningMessage string               `json:"transitioningMessage,omitempty"`
	Uuid                 string               `json:"uuid,omitempty"`
}
type ProjectLoggingCollection struct {
	types.Collection
	Data   []ProjectLogging `json:"data,omitempty"`
	client *ProjectLoggingClient
}

type ProjectLoggingClient struct {
	apiClient *Client
}

type ProjectLoggingOperations interface {
	List(opts *types.ListOpts) (*ProjectLoggingCollection, error)
	Create(opts *ProjectLogging) (*ProjectLogging, error)
	Update(existing *ProjectLogging, updates interface{}) (*ProjectLogging, error)
	ByID(id string) (*ProjectLogging, error)
	Delete(container *ProjectLogging) error
}

func newProjectLoggingClient(apiClient *Client) *ProjectLoggingClient {
	return &ProjectLoggingClient{
		apiClient: apiClient,
	}
}

func (c *ProjectLoggingClient) Create(container *ProjectLogging) (*ProjectLogging, error) {
	resp := &ProjectLogging{}
	err := c.apiClient.Ops.DoCreate(ProjectLoggingType, container, resp)
	return resp, err
}

func (c *ProjectLoggingClient) Update(existing *ProjectLogging, updates interface{}) (*ProjectLogging, error) {
	resp := &ProjectLogging{}
	err := c.apiClient.Ops.DoUpdate(ProjectLoggingType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectLoggingClient) List(opts *types.ListOpts) (*ProjectLoggingCollection, error) {
	resp := &ProjectLoggingCollection{}
	err := c.apiClient.Ops.DoList(ProjectLoggingType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ProjectLoggingCollection) Next() (*ProjectLoggingCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectLoggingCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectLoggingClient) ByID(id string) (*ProjectLogging, error) {
	resp := &ProjectLogging{}
	err := c.apiClient.Ops.DoByID(ProjectLoggingType, id, resp)
	return resp, err
}

func (c *ProjectLoggingClient) Delete(container *ProjectLogging) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectLoggingType, &container.Resource)
}
