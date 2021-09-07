package client

import (
	"github.com/rancher/norman/types"
)

const (
	ComposeConfigType                      = "composeConfig"
	ComposeConfigFieldAnnotations          = "annotations"
	ComposeConfigFieldCreated              = "created"
	ComposeConfigFieldCreatorID            = "creatorId"
	ComposeConfigFieldLabels               = "labels"
	ComposeConfigFieldName                 = "name"
	ComposeConfigFieldOwnerReferences      = "ownerReferences"
	ComposeConfigFieldRancherCompose       = "rancherCompose"
	ComposeConfigFieldRemoved              = "removed"
	ComposeConfigFieldState                = "state"
	ComposeConfigFieldStatus               = "status"
	ComposeConfigFieldTransitioning        = "transitioning"
	ComposeConfigFieldTransitioningMessage = "transitioningMessage"
	ComposeConfigFieldUUID                 = "uuid"
)

type ComposeConfig struct {
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
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ComposeConfigCollection struct {
	types.Collection
	Data   []ComposeConfig `json:"data,omitempty"`
	client *ComposeConfigClient
}

type ComposeConfigClient struct {
	apiClient *Client
}

type ComposeConfigOperations interface {
	List(opts *types.ListOpts) (*ComposeConfigCollection, error)
	ListAll(opts *types.ListOpts) (*ComposeConfigCollection, error)
	Create(opts *ComposeConfig) (*ComposeConfig, error)
	Update(existing *ComposeConfig, updates interface{}) (*ComposeConfig, error)
	Replace(existing *ComposeConfig) (*ComposeConfig, error)
	ByID(id string) (*ComposeConfig, error)
	Delete(container *ComposeConfig) error
}

func newComposeConfigClient(apiClient *Client) *ComposeConfigClient {
	return &ComposeConfigClient{
		apiClient: apiClient,
	}
}

func (c *ComposeConfigClient) Create(container *ComposeConfig) (*ComposeConfig, error) {
	resp := &ComposeConfig{}
	err := c.apiClient.Ops.DoCreate(ComposeConfigType, container, resp)
	return resp, err
}

func (c *ComposeConfigClient) Update(existing *ComposeConfig, updates interface{}) (*ComposeConfig, error) {
	resp := &ComposeConfig{}
	err := c.apiClient.Ops.DoUpdate(ComposeConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ComposeConfigClient) Replace(obj *ComposeConfig) (*ComposeConfig, error) {
	resp := &ComposeConfig{}
	err := c.apiClient.Ops.DoReplace(ComposeConfigType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ComposeConfigClient) List(opts *types.ListOpts) (*ComposeConfigCollection, error) {
	resp := &ComposeConfigCollection{}
	err := c.apiClient.Ops.DoList(ComposeConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ComposeConfigClient) ListAll(opts *types.ListOpts) (*ComposeConfigCollection, error) {
	resp := &ComposeConfigCollection{}
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

func (cc *ComposeConfigCollection) Next() (*ComposeConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ComposeConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ComposeConfigClient) ByID(id string) (*ComposeConfig, error) {
	resp := &ComposeConfig{}
	err := c.apiClient.Ops.DoByID(ComposeConfigType, id, resp)
	return resp, err
}

func (c *ComposeConfigClient) Delete(container *ComposeConfig) error {
	return c.apiClient.Ops.DoResourceDelete(ComposeConfigType, &container.Resource)
}
