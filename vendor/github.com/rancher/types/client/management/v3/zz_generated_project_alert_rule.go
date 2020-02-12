package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectAlertRuleType                       = "projectAlertRule"
	ProjectAlertRuleFieldAlertState            = "alertState"
	ProjectAlertRuleFieldAnnotations           = "annotations"
	ProjectAlertRuleFieldCreated               = "created"
	ProjectAlertRuleFieldCreatorID             = "creatorId"
	ProjectAlertRuleFieldGroupID               = "groupId"
	ProjectAlertRuleFieldGroupIntervalSeconds  = "groupIntervalSeconds"
	ProjectAlertRuleFieldGroupWaitSeconds      = "groupWaitSeconds"
	ProjectAlertRuleFieldInherited             = "inherited"
	ProjectAlertRuleFieldLabels                = "labels"
	ProjectAlertRuleFieldMetricRule            = "metricRule"
	ProjectAlertRuleFieldName                  = "name"
	ProjectAlertRuleFieldNamespaceId           = "namespaceId"
	ProjectAlertRuleFieldOwnerReferences       = "ownerReferences"
	ProjectAlertRuleFieldPodRule               = "podRule"
	ProjectAlertRuleFieldProjectID             = "projectId"
	ProjectAlertRuleFieldRemoved               = "removed"
	ProjectAlertRuleFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ProjectAlertRuleFieldSeverity              = "severity"
	ProjectAlertRuleFieldState                 = "state"
	ProjectAlertRuleFieldTransitioning         = "transitioning"
	ProjectAlertRuleFieldTransitioningMessage  = "transitioningMessage"
	ProjectAlertRuleFieldUUID                  = "uuid"
	ProjectAlertRuleFieldWorkloadRule          = "workloadRule"
)

type ProjectAlertRule struct {
	types.Resource
	AlertState            string            `json:"alertState,omitempty" yaml:"alertState,omitempty"`
	Annotations           map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created               string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID             string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	GroupID               string            `json:"groupId,omitempty" yaml:"groupId,omitempty"`
	GroupIntervalSeconds  int64             `json:"groupIntervalSeconds,omitempty" yaml:"groupIntervalSeconds,omitempty"`
	GroupWaitSeconds      int64             `json:"groupWaitSeconds,omitempty" yaml:"groupWaitSeconds,omitempty"`
	Inherited             *bool             `json:"inherited,omitempty" yaml:"inherited,omitempty"`
	Labels                map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	MetricRule            *MetricRule       `json:"metricRule,omitempty" yaml:"metricRule,omitempty"`
	Name                  string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId           string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences       []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PodRule               *PodRule          `json:"podRule,omitempty" yaml:"podRule,omitempty"`
	ProjectID             string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed               string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	RepeatIntervalSeconds int64             `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	Severity              string            `json:"severity,omitempty" yaml:"severity,omitempty"`
	State                 string            `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning         string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage  string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                  string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	WorkloadRule          *WorkloadRule     `json:"workloadRule,omitempty" yaml:"workloadRule,omitempty"`
}

type ProjectAlertRuleCollection struct {
	types.Collection
	Data   []ProjectAlertRule `json:"data,omitempty"`
	client *ProjectAlertRuleClient
}

type ProjectAlertRuleClient struct {
	apiClient *Client
}

type ProjectAlertRuleOperations interface {
	List(opts *types.ListOpts) (*ProjectAlertRuleCollection, error)
	ListAll(opts *types.ListOpts) (*ProjectAlertRuleCollection, error)
	Create(opts *ProjectAlertRule) (*ProjectAlertRule, error)
	Update(existing *ProjectAlertRule, updates interface{}) (*ProjectAlertRule, error)
	Replace(existing *ProjectAlertRule) (*ProjectAlertRule, error)
	ByID(id string) (*ProjectAlertRule, error)
	Delete(container *ProjectAlertRule) error

	ActionActivate(resource *ProjectAlertRule) error

	ActionDeactivate(resource *ProjectAlertRule) error

	ActionMute(resource *ProjectAlertRule) error

	ActionUnmute(resource *ProjectAlertRule) error
}

func newProjectAlertRuleClient(apiClient *Client) *ProjectAlertRuleClient {
	return &ProjectAlertRuleClient{
		apiClient: apiClient,
	}
}

func (c *ProjectAlertRuleClient) Create(container *ProjectAlertRule) (*ProjectAlertRule, error) {
	resp := &ProjectAlertRule{}
	err := c.apiClient.Ops.DoCreate(ProjectAlertRuleType, container, resp)
	return resp, err
}

func (c *ProjectAlertRuleClient) Update(existing *ProjectAlertRule, updates interface{}) (*ProjectAlertRule, error) {
	resp := &ProjectAlertRule{}
	err := c.apiClient.Ops.DoUpdate(ProjectAlertRuleType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectAlertRuleClient) Replace(obj *ProjectAlertRule) (*ProjectAlertRule, error) {
	resp := &ProjectAlertRule{}
	err := c.apiClient.Ops.DoReplace(ProjectAlertRuleType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ProjectAlertRuleClient) List(opts *types.ListOpts) (*ProjectAlertRuleCollection, error) {
	resp := &ProjectAlertRuleCollection{}
	err := c.apiClient.Ops.DoList(ProjectAlertRuleType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ProjectAlertRuleClient) ListAll(opts *types.ListOpts) (*ProjectAlertRuleCollection, error) {
	resp := &ProjectAlertRuleCollection{}
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

func (cc *ProjectAlertRuleCollection) Next() (*ProjectAlertRuleCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectAlertRuleCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectAlertRuleClient) ByID(id string) (*ProjectAlertRule, error) {
	resp := &ProjectAlertRule{}
	err := c.apiClient.Ops.DoByID(ProjectAlertRuleType, id, resp)
	return resp, err
}

func (c *ProjectAlertRuleClient) Delete(container *ProjectAlertRule) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectAlertRuleType, &container.Resource)
}

func (c *ProjectAlertRuleClient) ActionActivate(resource *ProjectAlertRule) error {
	err := c.apiClient.Ops.DoAction(ProjectAlertRuleType, "activate", &resource.Resource, nil, nil)
	return err
}

func (c *ProjectAlertRuleClient) ActionDeactivate(resource *ProjectAlertRule) error {
	err := c.apiClient.Ops.DoAction(ProjectAlertRuleType, "deactivate", &resource.Resource, nil, nil)
	return err
}

func (c *ProjectAlertRuleClient) ActionMute(resource *ProjectAlertRule) error {
	err := c.apiClient.Ops.DoAction(ProjectAlertRuleType, "mute", &resource.Resource, nil, nil)
	return err
}

func (c *ProjectAlertRuleClient) ActionUnmute(resource *ProjectAlertRule) error {
	err := c.apiClient.Ops.DoAction(ProjectAlertRuleType, "unmute", &resource.Resource, nil, nil)
	return err
}
