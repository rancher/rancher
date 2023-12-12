package client

import (
	"github.com/rancher/norman/types"
)

const (
	EncryptedTokenType                 = "encryptedToken"
	EncryptedTokenFieldAnnotations     = "annotations"
	EncryptedTokenFieldCreated         = "created"
	EncryptedTokenFieldCreatorID       = "creatorId"
	EncryptedTokenFieldExpiresAt       = "expiresAt"
	EncryptedTokenFieldLabels          = "labels"
	EncryptedTokenFieldName            = "name"
	EncryptedTokenFieldNamespaceId     = "namespaceId"
	EncryptedTokenFieldOwnerReferences = "ownerReferences"
	EncryptedTokenFieldRemoved         = "removed"
	EncryptedTokenFieldToken           = "token"
	EncryptedTokenFieldUUID            = "uuid"
)

type EncryptedToken struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ExpiresAt       string            `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Token           string            `json:"token,omitempty" yaml:"token,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type EncryptedTokenCollection struct {
	types.Collection
	Data   []EncryptedToken `json:"data,omitempty"`
	client *EncryptedTokenClient
}

type EncryptedTokenClient struct {
	apiClient *Client
}

type EncryptedTokenOperations interface {
	List(opts *types.ListOpts) (*EncryptedTokenCollection, error)
	ListAll(opts *types.ListOpts) (*EncryptedTokenCollection, error)
	Create(opts *EncryptedToken) (*EncryptedToken, error)
	Update(existing *EncryptedToken, updates interface{}) (*EncryptedToken, error)
	Replace(existing *EncryptedToken) (*EncryptedToken, error)
	ByID(id string) (*EncryptedToken, error)
	Delete(container *EncryptedToken) error
}

func newEncryptedTokenClient(apiClient *Client) *EncryptedTokenClient {
	return &EncryptedTokenClient{
		apiClient: apiClient,
	}
}

func (c *EncryptedTokenClient) Create(container *EncryptedToken) (*EncryptedToken, error) {
	resp := &EncryptedToken{}
	err := c.apiClient.Ops.DoCreate(EncryptedTokenType, container, resp)
	return resp, err
}

func (c *EncryptedTokenClient) Update(existing *EncryptedToken, updates interface{}) (*EncryptedToken, error) {
	resp := &EncryptedToken{}
	err := c.apiClient.Ops.DoUpdate(EncryptedTokenType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *EncryptedTokenClient) Replace(obj *EncryptedToken) (*EncryptedToken, error) {
	resp := &EncryptedToken{}
	err := c.apiClient.Ops.DoReplace(EncryptedTokenType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *EncryptedTokenClient) List(opts *types.ListOpts) (*EncryptedTokenCollection, error) {
	resp := &EncryptedTokenCollection{}
	err := c.apiClient.Ops.DoList(EncryptedTokenType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *EncryptedTokenClient) ListAll(opts *types.ListOpts) (*EncryptedTokenCollection, error) {
	resp := &EncryptedTokenCollection{}
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

func (cc *EncryptedTokenCollection) Next() (*EncryptedTokenCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &EncryptedTokenCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *EncryptedTokenClient) ByID(id string) (*EncryptedToken, error) {
	resp := &EncryptedToken{}
	err := c.apiClient.Ops.DoByID(EncryptedTokenType, id, resp)
	return resp, err
}

func (c *EncryptedTokenClient) Delete(container *EncryptedToken) error {
	return c.apiClient.Ops.DoResourceDelete(EncryptedTokenType, &container.Resource)
}
