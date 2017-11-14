package client

import (
	"github.com/rancher/norman/types"
)

const (
	LocalCredentialType                 = "localCredential"
	LocalCredentialFieldAnnotations     = "annotations"
	LocalCredentialFieldCreated         = "created"
	LocalCredentialFieldFinalizers      = "finalizers"
	LocalCredentialFieldLabels          = "labels"
	LocalCredentialFieldName            = "name"
	LocalCredentialFieldOwnerReferences = "ownerReferences"
	LocalCredentialFieldPassword        = "password"
	LocalCredentialFieldRemoved         = "removed"
	LocalCredentialFieldResourcePath    = "resourcePath"
	LocalCredentialFieldUsername        = "username"
	LocalCredentialFieldUuid            = "uuid"
)

type LocalCredential struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Password        string            `json:"password,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	ResourcePath    string            `json:"resourcePath,omitempty"`
	Username        string            `json:"username,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type LocalCredentialCollection struct {
	types.Collection
	Data   []LocalCredential `json:"data,omitempty"`
	client *LocalCredentialClient
}

type LocalCredentialClient struct {
	apiClient *Client
}

type LocalCredentialOperations interface {
	List(opts *types.ListOpts) (*LocalCredentialCollection, error)
	Create(opts *LocalCredential) (*LocalCredential, error)
	Update(existing *LocalCredential, updates interface{}) (*LocalCredential, error)
	ByID(id string) (*LocalCredential, error)
	Delete(container *LocalCredential) error
}

func newLocalCredentialClient(apiClient *Client) *LocalCredentialClient {
	return &LocalCredentialClient{
		apiClient: apiClient,
	}
}

func (c *LocalCredentialClient) Create(container *LocalCredential) (*LocalCredential, error) {
	resp := &LocalCredential{}
	err := c.apiClient.Ops.DoCreate(LocalCredentialType, container, resp)
	return resp, err
}

func (c *LocalCredentialClient) Update(existing *LocalCredential, updates interface{}) (*LocalCredential, error) {
	resp := &LocalCredential{}
	err := c.apiClient.Ops.DoUpdate(LocalCredentialType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *LocalCredentialClient) List(opts *types.ListOpts) (*LocalCredentialCollection, error) {
	resp := &LocalCredentialCollection{}
	err := c.apiClient.Ops.DoList(LocalCredentialType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *LocalCredentialCollection) Next() (*LocalCredentialCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &LocalCredentialCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *LocalCredentialClient) ByID(id string) (*LocalCredential, error) {
	resp := &LocalCredential{}
	err := c.apiClient.Ops.DoByID(LocalCredentialType, id, resp)
	return resp, err
}

func (c *LocalCredentialClient) Delete(container *LocalCredential) error {
	return c.apiClient.Ops.DoResourceDelete(LocalCredentialType, &container.Resource)
}
