package client

import (
	"github.com/rancher/norman/types"
)

const (
	SamlTokenType                 = "samlToken"
	SamlTokenFieldAnnotations     = "annotations"
	SamlTokenFieldCreated         = "created"
	SamlTokenFieldCreatorID       = "creatorId"
	SamlTokenFieldExpiresAt       = "expiresAt"
	SamlTokenFieldLabels          = "labels"
	SamlTokenFieldName            = "name"
	SamlTokenFieldNamespaceId     = "namespaceId"
	SamlTokenFieldOwnerReferences = "ownerReferences"
	SamlTokenFieldRemoved         = "removed"
	SamlTokenFieldToken           = "token"
	SamlTokenFieldUUID            = "uuid"
)

type SamlToken struct {
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

type SamlTokenCollection struct {
	types.Collection
	Data   []SamlToken `json:"data,omitempty"`
	client *SamlTokenClient
}

type SamlTokenClient struct {
	apiClient *Client
}

type SamlTokenOperations interface {
	List(opts *types.ListOpts) (*SamlTokenCollection, error)
	ListAll(opts *types.ListOpts) (*SamlTokenCollection, error)
	Create(opts *SamlToken) (*SamlToken, error)
	Update(existing *SamlToken, updates interface{}) (*SamlToken, error)
	Replace(existing *SamlToken) (*SamlToken, error)
	ByID(id string) (*SamlToken, error)
	Delete(container *SamlToken) error
}

func newSamlTokenClient(apiClient *Client) *SamlTokenClient {
	return &SamlTokenClient{
		apiClient: apiClient,
	}
}

func (c *SamlTokenClient) Create(container *SamlToken) (*SamlToken, error) {
	resp := &SamlToken{}
	err := c.apiClient.Ops.DoCreate(SamlTokenType, container, resp)
	return resp, err
}

func (c *SamlTokenClient) Update(existing *SamlToken, updates interface{}) (*SamlToken, error) {
	resp := &SamlToken{}
	err := c.apiClient.Ops.DoUpdate(SamlTokenType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *SamlTokenClient) Replace(obj *SamlToken) (*SamlToken, error) {
	resp := &SamlToken{}
	err := c.apiClient.Ops.DoReplace(SamlTokenType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *SamlTokenClient) List(opts *types.ListOpts) (*SamlTokenCollection, error) {
	resp := &SamlTokenCollection{}
	err := c.apiClient.Ops.DoList(SamlTokenType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *SamlTokenClient) ListAll(opts *types.ListOpts) (*SamlTokenCollection, error) {
	resp := &SamlTokenCollection{}
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

func (cc *SamlTokenCollection) Next() (*SamlTokenCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &SamlTokenCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *SamlTokenClient) ByID(id string) (*SamlToken, error) {
	resp := &SamlToken{}
	err := c.apiClient.Ops.DoByID(SamlTokenType, id, resp)
	return resp, err
}

func (c *SamlTokenClient) Delete(container *SamlToken) error {
	return c.apiClient.Ops.DoResourceDelete(SamlTokenType, &container.Resource)
}
