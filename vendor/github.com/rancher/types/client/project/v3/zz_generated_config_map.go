package client

import (
	"github.com/rancher/norman/types"
)

const (
	ConfigMapType                 = "configMap"
	ConfigMapFieldAnnotations     = "annotations"
	ConfigMapFieldBinaryData      = "binaryData"
	ConfigMapFieldCreated         = "created"
	ConfigMapFieldCreatorID       = "creatorId"
	ConfigMapFieldData            = "data"
	ConfigMapFieldLabels          = "labels"
	ConfigMapFieldName            = "name"
	ConfigMapFieldNamespaceId     = "namespaceId"
	ConfigMapFieldOwnerReferences = "ownerReferences"
	ConfigMapFieldProjectID       = "projectId"
	ConfigMapFieldRemoved         = "removed"
	ConfigMapFieldUUID            = "uuid"
)

type ConfigMap struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	BinaryData      map[string]string `json:"binaryData,omitempty" yaml:"binaryData,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Data            map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ConfigMapCollection struct {
	types.Collection
	Data   []ConfigMap `json:"data,omitempty"`
	client *ConfigMapClient
}

type ConfigMapClient struct {
	apiClient *Client
}

type ConfigMapOperations interface {
	List(opts *types.ListOpts) (*ConfigMapCollection, error)
	Create(opts *ConfigMap) (*ConfigMap, error)
	Update(existing *ConfigMap, updates interface{}) (*ConfigMap, error)
	Replace(existing *ConfigMap) (*ConfigMap, error)
	ByID(id string) (*ConfigMap, error)
	Delete(container *ConfigMap) error
}

func newConfigMapClient(apiClient *Client) *ConfigMapClient {
	return &ConfigMapClient{
		apiClient: apiClient,
	}
}

func (c *ConfigMapClient) Create(container *ConfigMap) (*ConfigMap, error) {
	resp := &ConfigMap{}
	err := c.apiClient.Ops.DoCreate(ConfigMapType, container, resp)
	return resp, err
}

func (c *ConfigMapClient) Update(existing *ConfigMap, updates interface{}) (*ConfigMap, error) {
	resp := &ConfigMap{}
	err := c.apiClient.Ops.DoUpdate(ConfigMapType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ConfigMapClient) Replace(obj *ConfigMap) (*ConfigMap, error) {
	resp := &ConfigMap{}
	err := c.apiClient.Ops.DoReplace(ConfigMapType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ConfigMapClient) List(opts *types.ListOpts) (*ConfigMapCollection, error) {
	resp := &ConfigMapCollection{}
	err := c.apiClient.Ops.DoList(ConfigMapType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ConfigMapCollection) Next() (*ConfigMapCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ConfigMapCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ConfigMapClient) ByID(id string) (*ConfigMap, error) {
	resp := &ConfigMap{}
	err := c.apiClient.Ops.DoByID(ConfigMapType, id, resp)
	return resp, err
}

func (c *ConfigMapClient) Delete(container *ConfigMap) error {
	return c.apiClient.Ops.DoResourceDelete(ConfigMapType, &container.Resource)
}
