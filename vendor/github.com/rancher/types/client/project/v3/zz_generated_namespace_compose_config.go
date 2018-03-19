package client

import (
	"github.com/rancher/norman/types"
)

const (
	NamespaceComposeConfigType                      = "namespaceComposeConfig"
	NamespaceComposeConfigFieldAnnotations          = "annotations"
	NamespaceComposeConfigFieldCreated              = "created"
	NamespaceComposeConfigFieldCreatorID            = "creatorId"
	NamespaceComposeConfigFieldInstallNamespace     = "installNamespace"
	NamespaceComposeConfigFieldLabels               = "labels"
	NamespaceComposeConfigFieldName                 = "name"
	NamespaceComposeConfigFieldNamespaceId          = "namespaceId"
	NamespaceComposeConfigFieldOwnerReferences      = "ownerReferences"
	NamespaceComposeConfigFieldProjectId            = "projectId"
	NamespaceComposeConfigFieldRancherCompose       = "rancherCompose"
	NamespaceComposeConfigFieldRemoved              = "removed"
	NamespaceComposeConfigFieldState                = "state"
	NamespaceComposeConfigFieldStatus               = "status"
	NamespaceComposeConfigFieldTransitioning        = "transitioning"
	NamespaceComposeConfigFieldTransitioningMessage = "transitioningMessage"
	NamespaceComposeConfigFieldUuid                 = "uuid"
)

type NamespaceComposeConfig struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	InstallNamespace     string            `json:"installNamespace,omitempty" yaml:"installNamespace,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectId            string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	RancherCompose       string            `json:"rancherCompose,omitempty" yaml:"rancherCompose,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *ComposeStatus    `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Uuid                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type NamespaceComposeConfigCollection struct {
	types.Collection
	Data   []NamespaceComposeConfig `json:"data,omitempty"`
	client *NamespaceComposeConfigClient
}

type NamespaceComposeConfigClient struct {
	apiClient *Client
}

type NamespaceComposeConfigOperations interface {
	List(opts *types.ListOpts) (*NamespaceComposeConfigCollection, error)
	Create(opts *NamespaceComposeConfig) (*NamespaceComposeConfig, error)
	Update(existing *NamespaceComposeConfig, updates interface{}) (*NamespaceComposeConfig, error)
	ByID(id string) (*NamespaceComposeConfig, error)
	Delete(container *NamespaceComposeConfig) error
}

func newNamespaceComposeConfigClient(apiClient *Client) *NamespaceComposeConfigClient {
	return &NamespaceComposeConfigClient{
		apiClient: apiClient,
	}
}

func (c *NamespaceComposeConfigClient) Create(container *NamespaceComposeConfig) (*NamespaceComposeConfig, error) {
	resp := &NamespaceComposeConfig{}
	err := c.apiClient.Ops.DoCreate(NamespaceComposeConfigType, container, resp)
	return resp, err
}

func (c *NamespaceComposeConfigClient) Update(existing *NamespaceComposeConfig, updates interface{}) (*NamespaceComposeConfig, error) {
	resp := &NamespaceComposeConfig{}
	err := c.apiClient.Ops.DoUpdate(NamespaceComposeConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NamespaceComposeConfigClient) List(opts *types.ListOpts) (*NamespaceComposeConfigCollection, error) {
	resp := &NamespaceComposeConfigCollection{}
	err := c.apiClient.Ops.DoList(NamespaceComposeConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *NamespaceComposeConfigCollection) Next() (*NamespaceComposeConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NamespaceComposeConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NamespaceComposeConfigClient) ByID(id string) (*NamespaceComposeConfig, error) {
	resp := &NamespaceComposeConfig{}
	err := c.apiClient.Ops.DoByID(NamespaceComposeConfigType, id, resp)
	return resp, err
}

func (c *NamespaceComposeConfigClient) Delete(container *NamespaceComposeConfig) error {
	return c.apiClient.Ops.DoResourceDelete(NamespaceComposeConfigType, &container.Resource)
}
