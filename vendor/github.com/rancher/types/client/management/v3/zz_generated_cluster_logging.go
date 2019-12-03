package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterLoggingType                        = "clusterLogging"
	ClusterLoggingFieldAnnotations            = "annotations"
	ClusterLoggingFieldAppliedSpec            = "appliedSpec"
	ClusterLoggingFieldClusterID              = "clusterId"
	ClusterLoggingFieldConditions             = "conditions"
	ClusterLoggingFieldCreated                = "created"
	ClusterLoggingFieldCreatorID              = "creatorId"
	ClusterLoggingFieldCustomTargetConfig     = "customTargetConfig"
	ClusterLoggingFieldElasticsearchConfig    = "elasticsearchConfig"
	ClusterLoggingFieldEnableJSONParsing      = "enableJSONParsing"
	ClusterLoggingFieldFailedSpec             = "failedSpec"
	ClusterLoggingFieldFluentForwarderConfig  = "fluentForwarderConfig"
	ClusterLoggingFieldIncludeSystemComponent = "includeSystemComponent"
	ClusterLoggingFieldKafkaConfig            = "kafkaConfig"
	ClusterLoggingFieldLabels                 = "labels"
	ClusterLoggingFieldName                   = "name"
	ClusterLoggingFieldNamespaceId            = "namespaceId"
	ClusterLoggingFieldOutputFlushInterval    = "outputFlushInterval"
	ClusterLoggingFieldOutputTags             = "outputTags"
	ClusterLoggingFieldOwnerReferences        = "ownerReferences"
	ClusterLoggingFieldRemoved                = "removed"
	ClusterLoggingFieldSplunkConfig           = "splunkConfig"
	ClusterLoggingFieldState                  = "state"
	ClusterLoggingFieldSyslogConfig           = "syslogConfig"
	ClusterLoggingFieldTransitioning          = "transitioning"
	ClusterLoggingFieldTransitioningMessage   = "transitioningMessage"
	ClusterLoggingFieldUUID                   = "uuid"
)

type ClusterLogging struct {
	types.Resource
	Annotations            map[string]string      `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AppliedSpec            *ClusterLoggingSpec    `json:"appliedSpec,omitempty" yaml:"appliedSpec,omitempty"`
	ClusterID              string                 `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Conditions             []LoggingCondition     `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created                string                 `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID              string                 `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	CustomTargetConfig     *CustomTargetConfig    `json:"customTargetConfig,omitempty" yaml:"customTargetConfig,omitempty"`
	ElasticsearchConfig    *ElasticsearchConfig   `json:"elasticsearchConfig,omitempty" yaml:"elasticsearchConfig,omitempty"`
	EnableJSONParsing      bool                   `json:"enableJSONParsing,omitempty" yaml:"enableJSONParsing,omitempty"`
	FailedSpec             *ClusterLoggingSpec    `json:"failedSpec,omitempty" yaml:"failedSpec,omitempty"`
	FluentForwarderConfig  *FluentForwarderConfig `json:"fluentForwarderConfig,omitempty" yaml:"fluentForwarderConfig,omitempty"`
	IncludeSystemComponent *bool                  `json:"includeSystemComponent,omitempty" yaml:"includeSystemComponent,omitempty"`
	KafkaConfig            *KafkaConfig           `json:"kafkaConfig,omitempty" yaml:"kafkaConfig,omitempty"`
	Labels                 map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                   string                 `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId            string                 `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OutputFlushInterval    int64                  `json:"outputFlushInterval,omitempty" yaml:"outputFlushInterval,omitempty"`
	OutputTags             map[string]string      `json:"outputTags,omitempty" yaml:"outputTags,omitempty"`
	OwnerReferences        []OwnerReference       `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed                string                 `json:"removed,omitempty" yaml:"removed,omitempty"`
	SplunkConfig           *SplunkConfig          `json:"splunkConfig,omitempty" yaml:"splunkConfig,omitempty"`
	State                  string                 `json:"state,omitempty" yaml:"state,omitempty"`
	SyslogConfig           *SyslogConfig          `json:"syslogConfig,omitempty" yaml:"syslogConfig,omitempty"`
	Transitioning          string                 `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage   string                 `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                   string                 `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ClusterLoggingCollection struct {
	types.Collection
	Data   []ClusterLogging `json:"data,omitempty"`
	client *ClusterLoggingClient
}

type ClusterLoggingClient struct {
	apiClient *Client
}

type ClusterLoggingOperations interface {
	List(opts *types.ListOpts) (*ClusterLoggingCollection, error)
	Create(opts *ClusterLogging) (*ClusterLogging, error)
	Update(existing *ClusterLogging, updates interface{}) (*ClusterLogging, error)
	Replace(existing *ClusterLogging) (*ClusterLogging, error)
	ByID(id string) (*ClusterLogging, error)
	Delete(container *ClusterLogging) error

	CollectionActionDryRun(resource *ClusterLoggingCollection, input *ClusterTestInput) error

	CollectionActionTest(resource *ClusterLoggingCollection, input *ClusterTestInput) error
}

func newClusterLoggingClient(apiClient *Client) *ClusterLoggingClient {
	return &ClusterLoggingClient{
		apiClient: apiClient,
	}
}

func (c *ClusterLoggingClient) Create(container *ClusterLogging) (*ClusterLogging, error) {
	resp := &ClusterLogging{}
	err := c.apiClient.Ops.DoCreate(ClusterLoggingType, container, resp)
	return resp, err
}

func (c *ClusterLoggingClient) Update(existing *ClusterLogging, updates interface{}) (*ClusterLogging, error) {
	resp := &ClusterLogging{}
	err := c.apiClient.Ops.DoUpdate(ClusterLoggingType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterLoggingClient) Replace(obj *ClusterLogging) (*ClusterLogging, error) {
	resp := &ClusterLogging{}
	err := c.apiClient.Ops.DoReplace(ClusterLoggingType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterLoggingClient) List(opts *types.ListOpts) (*ClusterLoggingCollection, error) {
	resp := &ClusterLoggingCollection{}
	err := c.apiClient.Ops.DoList(ClusterLoggingType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterLoggingCollection) Next() (*ClusterLoggingCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterLoggingCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterLoggingClient) ByID(id string) (*ClusterLogging, error) {
	resp := &ClusterLogging{}
	err := c.apiClient.Ops.DoByID(ClusterLoggingType, id, resp)
	return resp, err
}

func (c *ClusterLoggingClient) Delete(container *ClusterLogging) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterLoggingType, &container.Resource)
}

func (c *ClusterLoggingClient) CollectionActionDryRun(resource *ClusterLoggingCollection, input *ClusterTestInput) error {
	err := c.apiClient.Ops.DoCollectionAction(ClusterLoggingType, "dryRun", &resource.Collection, input, nil)
	return err
}

func (c *ClusterLoggingClient) CollectionActionTest(resource *ClusterLoggingCollection, input *ClusterTestInput) error {
	err := c.apiClient.Ops.DoCollectionAction(ClusterLoggingType, "test", &resource.Collection, input, nil)
	return err
}
