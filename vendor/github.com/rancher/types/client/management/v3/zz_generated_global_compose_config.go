package client

import (
	"github.com/rancher/norman/types"
)

const (
	GlobalComposeConfigType                      = "globalComposeConfig"
	GlobalComposeConfigFieldAnnotations          = "annotations"
	GlobalComposeConfigFieldCreated              = "created"
	GlobalComposeConfigFieldCreatorID            = "creatorId"
	GlobalComposeConfigFieldLabels               = "labels"
	GlobalComposeConfigFieldName                 = "name"
	GlobalComposeConfigFieldOwnerReferences      = "ownerReferences"
	GlobalComposeConfigFieldRancherCompose       = "rancherCompose"
	GlobalComposeConfigFieldRemoved              = "removed"
	GlobalComposeConfigFieldState                = "state"
	GlobalComposeConfigFieldStatus               = "status"
	GlobalComposeConfigFieldTransitioning        = "transitioning"
	GlobalComposeConfigFieldTransitioningMessage = "transitioningMessage"
	GlobalComposeConfigFieldUuid                 = "uuid"
)

type GlobalComposeConfig struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RancherCompose       string            `json:"rancherCompose,omitempty" yaml:"rancherCompose,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *ComposeStatus    `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Uuid                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type GlobalComposeConfigCollection struct {
	types.Collection
	Data   []GlobalComposeConfig `json:"data,omitempty"`
	client *GlobalComposeConfigClient
}

type GlobalComposeConfigClient struct {
	apiClient *Client
}

type GlobalComposeConfigOperations interface {
	List(opts *types.ListOpts) (*GlobalComposeConfigCollection, error)
	Create(opts *GlobalComposeConfig) (*GlobalComposeConfig, error)
	Update(existing *GlobalComposeConfig, updates interface{}) (*GlobalComposeConfig, error)
	ByID(id string) (*GlobalComposeConfig, error)
	Delete(container *GlobalComposeConfig) error
}

func newGlobalComposeConfigClient(apiClient *Client) *GlobalComposeConfigClient {
	return &GlobalComposeConfigClient{
		apiClient: apiClient,
	}
}

func (c *GlobalComposeConfigClient) Create(container *GlobalComposeConfig) (*GlobalComposeConfig, error) {
	resp := &GlobalComposeConfig{}
	err := c.apiClient.Ops.DoCreate(GlobalComposeConfigType, container, resp)
	return resp, err
}

func (c *GlobalComposeConfigClient) Update(existing *GlobalComposeConfig, updates interface{}) (*GlobalComposeConfig, error) {
	resp := &GlobalComposeConfig{}
	err := c.apiClient.Ops.DoUpdate(GlobalComposeConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GlobalComposeConfigClient) List(opts *types.ListOpts) (*GlobalComposeConfigCollection, error) {
	resp := &GlobalComposeConfigCollection{}
	err := c.apiClient.Ops.DoList(GlobalComposeConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *GlobalComposeConfigCollection) Next() (*GlobalComposeConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GlobalComposeConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GlobalComposeConfigClient) ByID(id string) (*GlobalComposeConfig, error) {
	resp := &GlobalComposeConfig{}
	err := c.apiClient.Ops.DoByID(GlobalComposeConfigType, id, resp)
	return resp, err
}

func (c *GlobalComposeConfigClient) Delete(container *GlobalComposeConfig) error {
	return c.apiClient.Ops.DoResourceDelete(GlobalComposeConfigType, &container.Resource)
}
