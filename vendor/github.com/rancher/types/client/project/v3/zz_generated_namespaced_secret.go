package client

import (
	"github.com/rancher/norman/types"
)

const (
	NamespacedSecretType                 = "namespacedSecret"
	NamespacedSecretField                = "creatorId"
	NamespacedSecretFieldAnnotations     = "annotations"
	NamespacedSecretFieldCreated         = "created"
	NamespacedSecretFieldData            = "data"
	NamespacedSecretFieldFinalizers      = "finalizers"
	NamespacedSecretFieldKind            = "kind"
	NamespacedSecretFieldLabels          = "labels"
	NamespacedSecretFieldName            = "name"
	NamespacedSecretFieldNamespaceId     = "namespaceId"
	NamespacedSecretFieldOwnerReferences = "ownerReferences"
	NamespacedSecretFieldProjectID       = "projectId"
	NamespacedSecretFieldRemoved         = "removed"
	NamespacedSecretFieldStringData      = "stringData"
	NamespacedSecretFieldUuid            = "uuid"
)

type NamespacedSecret struct {
	types.Resource
	string          `json:"creatorId,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	Data            map[string]string `json:"data,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Kind            string            `json:"kind,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	StringData      map[string]string `json:"stringData,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
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
