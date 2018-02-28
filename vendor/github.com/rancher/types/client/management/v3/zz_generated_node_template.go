package client

import (
	"github.com/rancher/norman/types"
)

const (
	NodeTemplateType                          = "nodeTemplate"
	NodeTemplateFieldAnnotations              = "annotations"
	NodeTemplateFieldAuthCertificateAuthority = "authCertificateAuthority"
	NodeTemplateFieldAuthKey                  = "authKey"
	NodeTemplateFieldCreated                  = "created"
	NodeTemplateFieldCreatorID                = "creatorId"
	NodeTemplateFieldDescription              = "description"
	NodeTemplateFieldDockerVersion            = "dockerVersion"
	NodeTemplateFieldDriver                   = "driver"
	NodeTemplateFieldEngineEnv                = "engineEnv"
	NodeTemplateFieldEngineInsecureRegistry   = "engineInsecureRegistry"
	NodeTemplateFieldEngineInstallURL         = "engineInstallURL"
	NodeTemplateFieldEngineLabel              = "engineLabel"
	NodeTemplateFieldEngineOpt                = "engineOpt"
	NodeTemplateFieldEngineRegistryMirror     = "engineRegistryMirror"
	NodeTemplateFieldEngineStorageDriver      = "engineStorageDriver"
	NodeTemplateFieldLabels                   = "labels"
	NodeTemplateFieldName                     = "name"
	NodeTemplateFieldOwnerReferences          = "ownerReferences"
	NodeTemplateFieldRemoved                  = "removed"
	NodeTemplateFieldState                    = "state"
	NodeTemplateFieldStatus                   = "status"
	NodeTemplateFieldTransitioning            = "transitioning"
	NodeTemplateFieldTransitioningMessage     = "transitioningMessage"
	NodeTemplateFieldUseInternalIPAddress     = "useInternalIpAddress"
	NodeTemplateFieldUuid                     = "uuid"
)

type NodeTemplate struct {
	types.Resource
	Annotations              map[string]string   `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthCertificateAuthority string              `json:"authCertificateAuthority,omitempty" yaml:"authCertificateAuthority,omitempty"`
	AuthKey                  string              `json:"authKey,omitempty" yaml:"authKey,omitempty"`
	Created                  string              `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                string              `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description              string              `json:"description,omitempty" yaml:"description,omitempty"`
	DockerVersion            string              `json:"dockerVersion,omitempty" yaml:"dockerVersion,omitempty"`
	Driver                   string              `json:"driver,omitempty" yaml:"driver,omitempty"`
	EngineEnv                map[string]string   `json:"engineEnv,omitempty" yaml:"engineEnv,omitempty"`
	EngineInsecureRegistry   []string            `json:"engineInsecureRegistry,omitempty" yaml:"engineInsecureRegistry,omitempty"`
	EngineInstallURL         string              `json:"engineInstallURL,omitempty" yaml:"engineInstallURL,omitempty"`
	EngineLabel              map[string]string   `json:"engineLabel,omitempty" yaml:"engineLabel,omitempty"`
	EngineOpt                map[string]string   `json:"engineOpt,omitempty" yaml:"engineOpt,omitempty"`
	EngineRegistryMirror     []string            `json:"engineRegistryMirror,omitempty" yaml:"engineRegistryMirror,omitempty"`
	EngineStorageDriver      string              `json:"engineStorageDriver,omitempty" yaml:"engineStorageDriver,omitempty"`
	Labels                   map[string]string   `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                     string              `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences          []OwnerReference    `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed                  string              `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                    string              `json:"state,omitempty" yaml:"state,omitempty"`
	Status                   *NodeTemplateStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning            string              `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage     string              `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UseInternalIPAddress     bool                `json:"useInternalIpAddress,omitempty" yaml:"useInternalIpAddress,omitempty"`
	Uuid                     string              `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type NodeTemplateCollection struct {
	types.Collection
	Data   []NodeTemplate `json:"data,omitempty"`
	client *NodeTemplateClient
}

type NodeTemplateClient struct {
	apiClient *Client
}

type NodeTemplateOperations interface {
	List(opts *types.ListOpts) (*NodeTemplateCollection, error)
	Create(opts *NodeTemplate) (*NodeTemplate, error)
	Update(existing *NodeTemplate, updates interface{}) (*NodeTemplate, error)
	ByID(id string) (*NodeTemplate, error)
	Delete(container *NodeTemplate) error
}

func newNodeTemplateClient(apiClient *Client) *NodeTemplateClient {
	return &NodeTemplateClient{
		apiClient: apiClient,
	}
}

func (c *NodeTemplateClient) Create(container *NodeTemplate) (*NodeTemplate, error) {
	resp := &NodeTemplate{}
	err := c.apiClient.Ops.DoCreate(NodeTemplateType, container, resp)
	return resp, err
}

func (c *NodeTemplateClient) Update(existing *NodeTemplate, updates interface{}) (*NodeTemplate, error) {
	resp := &NodeTemplate{}
	err := c.apiClient.Ops.DoUpdate(NodeTemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NodeTemplateClient) List(opts *types.ListOpts) (*NodeTemplateCollection, error) {
	resp := &NodeTemplateCollection{}
	err := c.apiClient.Ops.DoList(NodeTemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *NodeTemplateCollection) Next() (*NodeTemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NodeTemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NodeTemplateClient) ByID(id string) (*NodeTemplate, error) {
	resp := &NodeTemplate{}
	err := c.apiClient.Ops.DoByID(NodeTemplateType, id, resp)
	return resp, err
}

func (c *NodeTemplateClient) Delete(container *NodeTemplate) error {
	return c.apiClient.Ops.DoResourceDelete(NodeTemplateType, &container.Resource)
}
