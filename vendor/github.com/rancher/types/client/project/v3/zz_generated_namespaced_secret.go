package client

import (
	"github.com/rancher/norman/types"
)

const (
	NamespacedSecretType                 = "namespacedSecret"
	NamespacedSecretFieldAnnotations     = "annotations"
	NamespacedSecretFieldCreated         = "created"
	NamespacedSecretFieldCreatorID       = "creatorId"
	NamespacedSecretFieldData            = "data"
	NamespacedSecretFieldDescription     = "description"
	NamespacedSecretFieldKind            = "kind"
	NamespacedSecretFieldLabels          = "labels"
	NamespacedSecretFieldName            = "name"
	NamespacedSecretFieldNamespaceId     = "namespaceId"
	NamespacedSecretFieldOwnerReferences = "ownerReferences"
	NamespacedSecretFieldProjectID       = "projectId"
	NamespacedSecretFieldRemoved         = "removed"
	NamespacedSecretFieldStringData      = "stringData"
	NamespacedSecretFieldUUID            = "uuid"
)

type NamespacedSecret struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Data            map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
	Kind            string            `json:"kind,omitempty" yaml:"kind,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	StringData      map[string]string `json:"stringData,omitempty" yaml:"stringData,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type NamespacedSecretCollection struct {
	types.Collection
	Data   []NamespacedSecret `json:"data,omitempty"`
	client *NamespacedSecretClient
}

type NamespacedSecretClient struct {
	apiClient *Client
}

type NamespacedSecretOperations interface {
	List(opts *types.ListOpts) (*NamespacedSecretCollection, error)
	Create(opts *NamespacedSecret) (*NamespacedSecret, error)
	Update(existing *NamespacedSecret, updates interface{}) (*NamespacedSecret, error)
	Replace(existing *NamespacedSecret) (*NamespacedSecret, error)
	ByID(id string) (*NamespacedSecret, error)
	Delete(container *NamespacedSecret) error
}

func newNamespacedSecretClient(apiClient *Client) *NamespacedSecretClient {
	return &NamespacedSecretClient{
		apiClient: apiClient,
	}
}

func (c *NamespacedSecretClient) Create(container *NamespacedSecret) (*NamespacedSecret, error) {
	resp := &NamespacedSecret{}
	err := c.apiClient.Ops.DoCreate(NamespacedSecretType, container, resp)
	return resp, err
}

func (c *NamespacedSecretClient) Update(existing *NamespacedSecret, updates interface{}) (*NamespacedSecret, error) {
	resp := &NamespacedSecret{}
	err := c.apiClient.Ops.DoUpdate(NamespacedSecretType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NamespacedSecretClient) Replace(obj *NamespacedSecret) (*NamespacedSecret, error) {
	resp := &NamespacedSecret{}
	err := c.apiClient.Ops.DoReplace(NamespacedSecretType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *NamespacedSecretClient) List(opts *types.ListOpts) (*NamespacedSecretCollection, error) {
	resp := &NamespacedSecretCollection{}
	err := c.apiClient.Ops.DoList(NamespacedSecretType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *NamespacedSecretCollection) Next() (*NamespacedSecretCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NamespacedSecretCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NamespacedSecretClient) ByID(id string) (*NamespacedSecret, error) {
	resp := &NamespacedSecret{}
	err := c.apiClient.Ops.DoByID(NamespacedSecretType, id, resp)
	return resp, err
}

func (c *NamespacedSecretClient) Delete(container *NamespacedSecret) error {
	return c.apiClient.Ops.DoResourceDelete(NamespacedSecretType, &container.Resource)
}
