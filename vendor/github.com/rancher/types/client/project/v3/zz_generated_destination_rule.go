package client

import (
	"github.com/rancher/norman/types"
)

const (
	DestinationRuleType                      = "destinationRule"
	DestinationRuleFieldAnnotations          = "annotations"
	DestinationRuleFieldCreated              = "created"
	DestinationRuleFieldCreatorID            = "creatorId"
	DestinationRuleFieldHost                 = "host"
	DestinationRuleFieldLabels               = "labels"
	DestinationRuleFieldName                 = "name"
	DestinationRuleFieldNamespaceId          = "namespaceId"
	DestinationRuleFieldOwnerReferences      = "ownerReferences"
	DestinationRuleFieldProjectID            = "projectId"
	DestinationRuleFieldRemoved              = "removed"
	DestinationRuleFieldState                = "state"
	DestinationRuleFieldStatus               = "status"
	DestinationRuleFieldSubsets              = "subsets"
	DestinationRuleFieldTrafficPolicy        = "trafficPolicy"
	DestinationRuleFieldTransitioning        = "transitioning"
	DestinationRuleFieldTransitioningMessage = "transitioningMessage"
	DestinationRuleFieldUUID                 = "uuid"
)

type DestinationRule struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Host                 string            `json:"host,omitempty" yaml:"host,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID            string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               interface{}       `json:"status,omitempty" yaml:"status,omitempty"`
	Subsets              []Subset          `json:"subsets,omitempty" yaml:"subsets,omitempty"`
	TrafficPolicy        *TrafficPolicy    `json:"trafficPolicy,omitempty" yaml:"trafficPolicy,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type DestinationRuleCollection struct {
	types.Collection
	Data   []DestinationRule `json:"data,omitempty"`
	client *DestinationRuleClient
}

type DestinationRuleClient struct {
	apiClient *Client
}

type DestinationRuleOperations interface {
	List(opts *types.ListOpts) (*DestinationRuleCollection, error)
	Create(opts *DestinationRule) (*DestinationRule, error)
	Update(existing *DestinationRule, updates interface{}) (*DestinationRule, error)
	Replace(existing *DestinationRule) (*DestinationRule, error)
	ByID(id string) (*DestinationRule, error)
	Delete(container *DestinationRule) error
}

func newDestinationRuleClient(apiClient *Client) *DestinationRuleClient {
	return &DestinationRuleClient{
		apiClient: apiClient,
	}
}

func (c *DestinationRuleClient) Create(container *DestinationRule) (*DestinationRule, error) {
	resp := &DestinationRule{}
	err := c.apiClient.Ops.DoCreate(DestinationRuleType, container, resp)
	return resp, err
}

func (c *DestinationRuleClient) Update(existing *DestinationRule, updates interface{}) (*DestinationRule, error) {
	resp := &DestinationRule{}
	err := c.apiClient.Ops.DoUpdate(DestinationRuleType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DestinationRuleClient) Replace(obj *DestinationRule) (*DestinationRule, error) {
	resp := &DestinationRule{}
	err := c.apiClient.Ops.DoReplace(DestinationRuleType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *DestinationRuleClient) List(opts *types.ListOpts) (*DestinationRuleCollection, error) {
	resp := &DestinationRuleCollection{}
	err := c.apiClient.Ops.DoList(DestinationRuleType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *DestinationRuleCollection) Next() (*DestinationRuleCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &DestinationRuleCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *DestinationRuleClient) ByID(id string) (*DestinationRule, error) {
	resp := &DestinationRule{}
	err := c.apiClient.Ops.DoByID(DestinationRuleType, id, resp)
	return resp, err
}

func (c *DestinationRuleClient) Delete(container *DestinationRule) error {
	return c.apiClient.Ops.DoResourceDelete(DestinationRuleType, &container.Resource)
}
