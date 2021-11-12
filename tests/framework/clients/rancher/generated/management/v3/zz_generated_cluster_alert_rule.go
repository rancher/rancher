package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterAlertRuleType                       = "clusterAlertRule"
	ClusterAlertRuleFieldAlertState            = "alertState"
	ClusterAlertRuleFieldAnnotations           = "annotations"
	ClusterAlertRuleFieldClusterID             = "clusterId"
	ClusterAlertRuleFieldClusterScanRule       = "clusterScanRule"
	ClusterAlertRuleFieldCreated               = "created"
	ClusterAlertRuleFieldCreatorID             = "creatorId"
	ClusterAlertRuleFieldEventRule             = "eventRule"
	ClusterAlertRuleFieldGroupID               = "groupId"
	ClusterAlertRuleFieldGroupIntervalSeconds  = "groupIntervalSeconds"
	ClusterAlertRuleFieldGroupWaitSeconds      = "groupWaitSeconds"
	ClusterAlertRuleFieldInherited             = "inherited"
	ClusterAlertRuleFieldLabels                = "labels"
	ClusterAlertRuleFieldMetricRule            = "metricRule"
	ClusterAlertRuleFieldName                  = "name"
	ClusterAlertRuleFieldNamespaceId           = "namespaceId"
	ClusterAlertRuleFieldNodeRule              = "nodeRule"
	ClusterAlertRuleFieldOwnerReferences       = "ownerReferences"
	ClusterAlertRuleFieldRemoved               = "removed"
	ClusterAlertRuleFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ClusterAlertRuleFieldSeverity              = "severity"
	ClusterAlertRuleFieldState                 = "state"
	ClusterAlertRuleFieldSystemServiceRule     = "systemServiceRule"
	ClusterAlertRuleFieldTransitioning         = "transitioning"
	ClusterAlertRuleFieldTransitioningMessage  = "transitioningMessage"
	ClusterAlertRuleFieldUUID                  = "uuid"
)

type ClusterAlertRule struct {
	types.Resource
	AlertState            string             `json:"alertState,omitempty" yaml:"alertState,omitempty"`
	Annotations           map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterID             string             `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ClusterScanRule       *ClusterScanRule   `json:"clusterScanRule,omitempty" yaml:"clusterScanRule,omitempty"`
	Created               string             `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID             string             `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	EventRule             *EventRule         `json:"eventRule,omitempty" yaml:"eventRule,omitempty"`
	GroupID               string             `json:"groupId,omitempty" yaml:"groupId,omitempty"`
	GroupIntervalSeconds  int64              `json:"groupIntervalSeconds,omitempty" yaml:"groupIntervalSeconds,omitempty"`
	GroupWaitSeconds      int64              `json:"groupWaitSeconds,omitempty" yaml:"groupWaitSeconds,omitempty"`
	Inherited             *bool              `json:"inherited,omitempty" yaml:"inherited,omitempty"`
	Labels                map[string]string  `json:"labels,omitempty" yaml:"labels,omitempty"`
	MetricRule            *MetricRule        `json:"metricRule,omitempty" yaml:"metricRule,omitempty"`
	Name                  string             `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId           string             `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeRule              *NodeRule          `json:"nodeRule,omitempty" yaml:"nodeRule,omitempty"`
	OwnerReferences       []OwnerReference   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed               string             `json:"removed,omitempty" yaml:"removed,omitempty"`
	RepeatIntervalSeconds int64              `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	Severity              string             `json:"severity,omitempty" yaml:"severity,omitempty"`
	State                 string             `json:"state,omitempty" yaml:"state,omitempty"`
	SystemServiceRule     *SystemServiceRule `json:"systemServiceRule,omitempty" yaml:"systemServiceRule,omitempty"`
	Transitioning         string             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage  string             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                  string             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ClusterAlertRuleCollection struct {
	types.Collection
	Data   []ClusterAlertRule `json:"data,omitempty"`
	client *ClusterAlertRuleClient
}

type ClusterAlertRuleClient struct {
	apiClient *Client
}

type ClusterAlertRuleOperations interface {
	List(opts *types.ListOpts) (*ClusterAlertRuleCollection, error)
	ListAll(opts *types.ListOpts) (*ClusterAlertRuleCollection, error)
	Create(opts *ClusterAlertRule) (*ClusterAlertRule, error)
	Update(existing *ClusterAlertRule, updates interface{}) (*ClusterAlertRule, error)
	Replace(existing *ClusterAlertRule) (*ClusterAlertRule, error)
	ByID(id string) (*ClusterAlertRule, error)
	Delete(container *ClusterAlertRule) error

	ActionActivate(resource *ClusterAlertRule) error

	ActionDeactivate(resource *ClusterAlertRule) error

	ActionMute(resource *ClusterAlertRule) error

	ActionUnmute(resource *ClusterAlertRule) error
}

func newClusterAlertRuleClient(apiClient *Client) *ClusterAlertRuleClient {
	return &ClusterAlertRuleClient{
		apiClient: apiClient,
	}
}

func (c *ClusterAlertRuleClient) Create(container *ClusterAlertRule) (*ClusterAlertRule, error) {
	resp := &ClusterAlertRule{}
	err := c.apiClient.Ops.DoCreate(ClusterAlertRuleType, container, resp)
	return resp, err
}

func (c *ClusterAlertRuleClient) Update(existing *ClusterAlertRule, updates interface{}) (*ClusterAlertRule, error) {
	resp := &ClusterAlertRule{}
	err := c.apiClient.Ops.DoUpdate(ClusterAlertRuleType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterAlertRuleClient) Replace(obj *ClusterAlertRule) (*ClusterAlertRule, error) {
	resp := &ClusterAlertRule{}
	err := c.apiClient.Ops.DoReplace(ClusterAlertRuleType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterAlertRuleClient) List(opts *types.ListOpts) (*ClusterAlertRuleCollection, error) {
	resp := &ClusterAlertRuleCollection{}
	err := c.apiClient.Ops.DoList(ClusterAlertRuleType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ClusterAlertRuleClient) ListAll(opts *types.ListOpts) (*ClusterAlertRuleCollection, error) {
	resp := &ClusterAlertRuleCollection{}
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

func (cc *ClusterAlertRuleCollection) Next() (*ClusterAlertRuleCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterAlertRuleCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterAlertRuleClient) ByID(id string) (*ClusterAlertRule, error) {
	resp := &ClusterAlertRule{}
	err := c.apiClient.Ops.DoByID(ClusterAlertRuleType, id, resp)
	return resp, err
}

func (c *ClusterAlertRuleClient) Delete(container *ClusterAlertRule) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterAlertRuleType, &container.Resource)
}

func (c *ClusterAlertRuleClient) ActionActivate(resource *ClusterAlertRule) error {
	err := c.apiClient.Ops.DoAction(ClusterAlertRuleType, "activate", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterAlertRuleClient) ActionDeactivate(resource *ClusterAlertRule) error {
	err := c.apiClient.Ops.DoAction(ClusterAlertRuleType, "deactivate", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterAlertRuleClient) ActionMute(resource *ClusterAlertRule) error {
	err := c.apiClient.Ops.DoAction(ClusterAlertRuleType, "mute", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterAlertRuleClient) ActionUnmute(resource *ClusterAlertRule) error {
	err := c.apiClient.Ops.DoAction(ClusterAlertRuleType, "unmute", &resource.Resource, nil, nil)
	return err
}
