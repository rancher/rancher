package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterComposeConfigType                      = "clusterComposeConfig"
	ClusterComposeConfigFieldAnnotations          = "annotations"
	ClusterComposeConfigFieldClusterId            = "clusterId"
	ClusterComposeConfigFieldCreated              = "created"
	ClusterComposeConfigFieldCreatorID            = "creatorId"
	ClusterComposeConfigFieldLabels               = "labels"
	ClusterComposeConfigFieldName                 = "name"
	ClusterComposeConfigFieldNamespaceId          = "namespaceId"
	ClusterComposeConfigFieldOwnerReferences      = "ownerReferences"
	ClusterComposeConfigFieldRancherCompose       = "rancherCompose"
	ClusterComposeConfigFieldRemoved              = "removed"
	ClusterComposeConfigFieldState                = "state"
	ClusterComposeConfigFieldStatus               = "status"
	ClusterComposeConfigFieldTransitioning        = "transitioning"
	ClusterComposeConfigFieldTransitioningMessage = "transitioningMessage"
	ClusterComposeConfigFieldUuid                 = "uuid"
)

type ClusterComposeConfig struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterId            string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RancherCompose       string            `json:"rancherCompose,omitempty" yaml:"rancherCompose,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *ComposeStatus    `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Uuid                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type ClusterComposeConfigCollection struct {
	types.Collection
	Data   []ClusterComposeConfig `json:"data,omitempty"`
	client *ClusterComposeConfigClient
}

type ClusterComposeConfigClient struct {
	apiClient *Client
}

type ClusterComposeConfigOperations interface {
	List(opts *types.ListOpts) (*ClusterComposeConfigCollection, error)
	Create(opts *ClusterComposeConfig) (*ClusterComposeConfig, error)
	Update(existing *ClusterComposeConfig, updates interface{}) (*ClusterComposeConfig, error)
	ByID(id string) (*ClusterComposeConfig, error)
	Delete(container *ClusterComposeConfig) error
}

func newClusterComposeConfigClient(apiClient *Client) *ClusterComposeConfigClient {
	return &ClusterComposeConfigClient{
		apiClient: apiClient,
	}
}

func (c *ClusterComposeConfigClient) Create(container *ClusterComposeConfig) (*ClusterComposeConfig, error) {
	resp := &ClusterComposeConfig{}
	err := c.apiClient.Ops.DoCreate(ClusterComposeConfigType, container, resp)
	return resp, err
}

func (c *ClusterComposeConfigClient) Update(existing *ClusterComposeConfig, updates interface{}) (*ClusterComposeConfig, error) {
	resp := &ClusterComposeConfig{}
	err := c.apiClient.Ops.DoUpdate(ClusterComposeConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterComposeConfigClient) List(opts *types.ListOpts) (*ClusterComposeConfigCollection, error) {
	resp := &ClusterComposeConfigCollection{}
	err := c.apiClient.Ops.DoList(ClusterComposeConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterComposeConfigCollection) Next() (*ClusterComposeConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterComposeConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterComposeConfigClient) ByID(id string) (*ClusterComposeConfig, error) {
	resp := &ClusterComposeConfig{}
	err := c.apiClient.Ops.DoByID(ClusterComposeConfigType, id, resp)
	return resp, err
}

func (c *ClusterComposeConfigClient) Delete(container *ClusterComposeConfig) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterComposeConfigType, &container.Resource)
}
