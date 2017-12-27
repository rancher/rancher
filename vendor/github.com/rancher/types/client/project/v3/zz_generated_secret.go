package client

import (
	"github.com/rancher/norman/types"
)

const (
	SecretType                 = "secret"
	SecretField                = "creatorId"
	SecretFieldAnnotations     = "annotations"
	SecretFieldCreated         = "created"
	SecretFieldData            = "data"
	SecretFieldFinalizers      = "finalizers"
	SecretFieldKind            = "kind"
	SecretFieldLabels          = "labels"
	SecretFieldName            = "name"
	SecretFieldNamespaceId     = "namespaceId"
	SecretFieldOwnerReferences = "ownerReferences"
	SecretFieldProjectID       = "projectId"
	SecretFieldRemoved         = "removed"
	SecretFieldStringData      = "stringData"
	SecretFieldUuid            = "uuid"
)

type Secret struct {
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
type SecretCollection struct {
	types.Collection
	Data   []Secret `json:"data,omitempty"`
	client *SecretClient
}

type SecretClient struct {
	apiClient *Client
}

type SecretOperations interface {
	List(opts *types.ListOpts) (*SecretCollection, error)
	Create(opts *Secret) (*Secret, error)
	Update(existing *Secret, updates interface{}) (*Secret, error)
	ByID(id string) (*Secret, error)
	Delete(container *Secret) error
}

func newSecretClient(apiClient *Client) *SecretClient {
	return &SecretClient{
		apiClient: apiClient,
	}
}

func (c *SecretClient) Create(container *Secret) (*Secret, error) {
	resp := &Secret{}
	err := c.apiClient.Ops.DoCreate(SecretType, container, resp)
	return resp, err
}

func (c *SecretClient) Update(existing *Secret, updates interface{}) (*Secret, error) {
	resp := &Secret{}
	err := c.apiClient.Ops.DoUpdate(SecretType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *SecretClient) List(opts *types.ListOpts) (*SecretCollection, error) {
	resp := &SecretCollection{}
	err := c.apiClient.Ops.DoList(SecretType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *SecretCollection) Next() (*SecretCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &SecretCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *SecretClient) ByID(id string) (*Secret, error) {
	resp := &Secret{}
	err := c.apiClient.Ops.DoByID(SecretType, id, resp)
	return resp, err
}

func (c *SecretClient) Delete(container *Secret) error {
	return c.apiClient.Ops.DoResourceDelete(SecretType, &container.Resource)
}
