package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterRandomizerType                      = "clusterRandomizer"
	ClusterRandomizerFieldAnnotations          = "annotations"
	ClusterRandomizerFieldCreated              = "created"
	ClusterRandomizerFieldCreatorID            = "creatorId"
	ClusterRandomizerFieldExampleString        = "rancherCompose"
	ClusterRandomizerFieldLabels               = "labels"
	ClusterRandomizerFieldName                 = "name"
	ClusterRandomizerFieldOwnerReferences      = "ownerReferences"
	ClusterRandomizerFieldRemoved              = "removed"
	ClusterRandomizerFieldState                = "state"
	ClusterRandomizerFieldStatus               = "status"
	ClusterRandomizerFieldTransitioning        = "transitioning"
	ClusterRandomizerFieldTransitioningMessage = "transitioningMessage"
	ClusterRandomizerFieldUUID                 = "uuid"
)

type ClusterRandomizer struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ExampleString        string            `json:"rancherCompose,omitempty" yaml:"rancherCompose,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *RandomizerStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ClusterRandomizerCollection struct {
	types.Collection
	Data   []ClusterRandomizer `json:"data,omitempty"`
	client *ClusterRandomizerClient
}

type ClusterRandomizerClient struct {
	apiClient *Client
}

type ClusterRandomizerOperations interface {
	List(opts *types.ListOpts) (*ClusterRandomizerCollection, error)
	Create(opts *ClusterRandomizer) (*ClusterRandomizer, error)
	Update(existing *ClusterRandomizer, updates interface{}) (*ClusterRandomizer, error)
	Replace(existing *ClusterRandomizer) (*ClusterRandomizer, error)
	ByID(id string) (*ClusterRandomizer, error)
	Delete(container *ClusterRandomizer) error
}

func newClusterRandomizerClient(apiClient *Client) *ClusterRandomizerClient {
	return &ClusterRandomizerClient{
		apiClient: apiClient,
	}
}

func (c *ClusterRandomizerClient) Create(container *ClusterRandomizer) (*ClusterRandomizer, error) {
	resp := &ClusterRandomizer{}
	err := c.apiClient.Ops.DoCreate(ClusterRandomizerType, container, resp)
	return resp, err
}

func (c *ClusterRandomizerClient) Update(existing *ClusterRandomizer, updates interface{}) (*ClusterRandomizer, error) {
	resp := &ClusterRandomizer{}
	err := c.apiClient.Ops.DoUpdate(ClusterRandomizerType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterRandomizerClient) Replace(obj *ClusterRandomizer) (*ClusterRandomizer, error) {
	resp := &ClusterRandomizer{}
	err := c.apiClient.Ops.DoReplace(ClusterRandomizerType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterRandomizerClient) List(opts *types.ListOpts) (*ClusterRandomizerCollection, error) {
	resp := &ClusterRandomizerCollection{}
	err := c.apiClient.Ops.DoList(ClusterRandomizerType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterRandomizerCollection) Next() (*ClusterRandomizerCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterRandomizerCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterRandomizerClient) ByID(id string) (*ClusterRandomizer, error) {
	resp := &ClusterRandomizer{}
	err := c.apiClient.Ops.DoByID(ClusterRandomizerType, id, resp)
	return resp, err
}

func (c *ClusterRandomizerClient) Delete(container *ClusterRandomizer) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterRandomizerType, &container.Resource)
}
