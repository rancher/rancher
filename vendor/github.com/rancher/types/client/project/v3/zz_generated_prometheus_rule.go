package client

import (
	"github.com/rancher/norman/types"
)

const (
	PrometheusRuleType                 = "prometheusRule"
	PrometheusRuleFieldAnnotations     = "annotations"
	PrometheusRuleFieldCreated         = "created"
	PrometheusRuleFieldCreatorID       = "creatorId"
	PrometheusRuleFieldGroups          = "groups"
	PrometheusRuleFieldLabels          = "labels"
	PrometheusRuleFieldName            = "name"
	PrometheusRuleFieldNamespaceId     = "namespaceId"
	PrometheusRuleFieldOwnerReferences = "ownerReferences"
	PrometheusRuleFieldProjectID       = "projectId"
	PrometheusRuleFieldRemoved         = "removed"
	PrometheusRuleFieldUUID            = "uuid"
)

type PrometheusRule struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Groups          []RuleGroup       `json:"groups,omitempty" yaml:"groups,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type PrometheusRuleCollection struct {
	types.Collection
	Data   []PrometheusRule `json:"data,omitempty"`
	client *PrometheusRuleClient
}

type PrometheusRuleClient struct {
	apiClient *Client
}

type PrometheusRuleOperations interface {
	List(opts *types.ListOpts) (*PrometheusRuleCollection, error)
	Create(opts *PrometheusRule) (*PrometheusRule, error)
	Update(existing *PrometheusRule, updates interface{}) (*PrometheusRule, error)
	Replace(existing *PrometheusRule) (*PrometheusRule, error)
	ByID(id string) (*PrometheusRule, error)
	Delete(container *PrometheusRule) error
}

func newPrometheusRuleClient(apiClient *Client) *PrometheusRuleClient {
	return &PrometheusRuleClient{
		apiClient: apiClient,
	}
}

func (c *PrometheusRuleClient) Create(container *PrometheusRule) (*PrometheusRule, error) {
	resp := &PrometheusRule{}
	err := c.apiClient.Ops.DoCreate(PrometheusRuleType, container, resp)
	return resp, err
}

func (c *PrometheusRuleClient) Update(existing *PrometheusRule, updates interface{}) (*PrometheusRule, error) {
	resp := &PrometheusRule{}
	err := c.apiClient.Ops.DoUpdate(PrometheusRuleType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PrometheusRuleClient) Replace(obj *PrometheusRule) (*PrometheusRule, error) {
	resp := &PrometheusRule{}
	err := c.apiClient.Ops.DoReplace(PrometheusRuleType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PrometheusRuleClient) List(opts *types.ListOpts) (*PrometheusRuleCollection, error) {
	resp := &PrometheusRuleCollection{}
	err := c.apiClient.Ops.DoList(PrometheusRuleType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *PrometheusRuleCollection) Next() (*PrometheusRuleCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PrometheusRuleCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PrometheusRuleClient) ByID(id string) (*PrometheusRule, error) {
	resp := &PrometheusRule{}
	err := c.apiClient.Ops.DoByID(PrometheusRuleType, id, resp)
	return resp, err
}

func (c *PrometheusRuleClient) Delete(container *PrometheusRule) error {
	return c.apiClient.Ops.DoResourceDelete(PrometheusRuleType, &container.Resource)
}
