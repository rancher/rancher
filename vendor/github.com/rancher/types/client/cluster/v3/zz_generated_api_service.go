package client

import (
	"github.com/rancher/norman/types"
)

const (
	APIServiceType                       = "apiService"
	APIServiceFieldAnnotations           = "annotations"
	APIServiceFieldCABundle              = "caBundle"
	APIServiceFieldConditions            = "conditions"
	APIServiceFieldCreated               = "created"
	APIServiceFieldCreatorID             = "creatorId"
	APIServiceFieldGroup                 = "group"
	APIServiceFieldGroupPriorityMinimum  = "groupPriorityMinimum"
	APIServiceFieldInsecureSkipTLSVerify = "insecureSkipTLSVerify"
	APIServiceFieldLabels                = "labels"
	APIServiceFieldName                  = "name"
	APIServiceFieldOwnerReferences       = "ownerReferences"
	APIServiceFieldRemoved               = "removed"
	APIServiceFieldService               = "service"
	APIServiceFieldState                 = "state"
	APIServiceFieldTransitioning         = "transitioning"
	APIServiceFieldTransitioningMessage  = "transitioningMessage"
	APIServiceFieldUUID                  = "uuid"
	APIServiceFieldVersion               = "version"
	APIServiceFieldVersionPriority       = "versionPriority"
)

type APIService struct {
	types.Resource
	Annotations           map[string]string     `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	CABundle              string                `json:"caBundle,omitempty" yaml:"caBundle,omitempty"`
	Conditions            []APIServiceCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created               string                `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID             string                `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Group                 string                `json:"group,omitempty" yaml:"group,omitempty"`
	GroupPriorityMinimum  int64                 `json:"groupPriorityMinimum,omitempty" yaml:"groupPriorityMinimum,omitempty"`
	InsecureSkipTLSVerify bool                  `json:"insecureSkipTLSVerify,omitempty" yaml:"insecureSkipTLSVerify,omitempty"`
	Labels                map[string]string     `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                  string                `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences       []OwnerReference      `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed               string                `json:"removed,omitempty" yaml:"removed,omitempty"`
	Service               *ServiceReference     `json:"service,omitempty" yaml:"service,omitempty"`
	State                 string                `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning         string                `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage  string                `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                  string                `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Version               string                `json:"version,omitempty" yaml:"version,omitempty"`
	VersionPriority       int64                 `json:"versionPriority,omitempty" yaml:"versionPriority,omitempty"`
}

type APIServiceCollection struct {
	types.Collection
	Data   []APIService `json:"data,omitempty"`
	client *APIServiceClient
}

type APIServiceClient struct {
	apiClient *Client
}

type APIServiceOperations interface {
	List(opts *types.ListOpts) (*APIServiceCollection, error)
	ListAll(opts *types.ListOpts) (*APIServiceCollection, error)
	Create(opts *APIService) (*APIService, error)
	Update(existing *APIService, updates interface{}) (*APIService, error)
	Replace(existing *APIService) (*APIService, error)
	ByID(id string) (*APIService, error)
	Delete(container *APIService) error
}

func newAPIServiceClient(apiClient *Client) *APIServiceClient {
	return &APIServiceClient{
		apiClient: apiClient,
	}
}

func (c *APIServiceClient) Create(container *APIService) (*APIService, error) {
	resp := &APIService{}
	err := c.apiClient.Ops.DoCreate(APIServiceType, container, resp)
	return resp, err
}

func (c *APIServiceClient) Update(existing *APIService, updates interface{}) (*APIService, error) {
	resp := &APIService{}
	err := c.apiClient.Ops.DoUpdate(APIServiceType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *APIServiceClient) Replace(obj *APIService) (*APIService, error) {
	resp := &APIService{}
	err := c.apiClient.Ops.DoReplace(APIServiceType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *APIServiceClient) List(opts *types.ListOpts) (*APIServiceCollection, error) {
	resp := &APIServiceCollection{}
	err := c.apiClient.Ops.DoList(APIServiceType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *APIServiceClient) ListAll(opts *types.ListOpts) (*APIServiceCollection, error) {
	resp := &APIServiceCollection{}
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

func (cc *APIServiceCollection) Next() (*APIServiceCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &APIServiceCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *APIServiceClient) ByID(id string) (*APIService, error) {
	resp := &APIService{}
	err := c.apiClient.Ops.DoByID(APIServiceType, id, resp)
	return resp, err
}

func (c *APIServiceClient) Delete(container *APIService) error {
	return c.apiClient.Ops.DoResourceDelete(APIServiceType, &container.Resource)
}
