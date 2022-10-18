package client

import (
	"github.com/rancher/norman/types"
)

const (
	ManagementSecretType                 = "managementSecret"
	ManagementSecretFieldAnnotations     = "annotations"
	ManagementSecretFieldCreated         = "created"
	ManagementSecretFieldCreatorID       = "creatorId"
	ManagementSecretFieldData            = "data"
	ManagementSecretFieldImmutable       = "immutable"
	ManagementSecretFieldLabels          = "labels"
	ManagementSecretFieldName            = "name"
	ManagementSecretFieldOwnerReferences = "ownerReferences"
	ManagementSecretFieldRemoved         = "removed"
	ManagementSecretFieldStringData      = "stringData"
	ManagementSecretFieldType            = "type"
	ManagementSecretFieldUUID            = "uuid"
)

type ManagementSecret struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Data            map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
	Immutable       *bool             `json:"immutable,omitempty" yaml:"immutable,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	StringData      map[string]string `json:"stringData,omitempty" yaml:"stringData,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ManagementSecretCollection struct {
	types.Collection
	Data   []ManagementSecret `json:"data,omitempty"`
	client *ManagementSecretClient
}

type ManagementSecretClient struct {
	apiClient *Client
}

type ManagementSecretOperations interface {
	List(opts *types.ListOpts) (*ManagementSecretCollection, error)
	ListAll(opts *types.ListOpts) (*ManagementSecretCollection, error)
	Create(opts *ManagementSecret) (*ManagementSecret, error)
	Update(existing *ManagementSecret, updates interface{}) (*ManagementSecret, error)
	Replace(existing *ManagementSecret) (*ManagementSecret, error)
	ByID(id string) (*ManagementSecret, error)
	Delete(container *ManagementSecret) error
}

func newManagementSecretClient(apiClient *Client) *ManagementSecretClient {
	return &ManagementSecretClient{
		apiClient: apiClient,
	}
}

func (c *ManagementSecretClient) Create(container *ManagementSecret) (*ManagementSecret, error) {
	resp := &ManagementSecret{}
	err := c.apiClient.Ops.DoCreate(ManagementSecretType, container, resp)
	return resp, err
}

func (c *ManagementSecretClient) Update(existing *ManagementSecret, updates interface{}) (*ManagementSecret, error) {
	resp := &ManagementSecret{}
	err := c.apiClient.Ops.DoUpdate(ManagementSecretType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ManagementSecretClient) Replace(obj *ManagementSecret) (*ManagementSecret, error) {
	resp := &ManagementSecret{}
	err := c.apiClient.Ops.DoReplace(ManagementSecretType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ManagementSecretClient) List(opts *types.ListOpts) (*ManagementSecretCollection, error) {
	resp := &ManagementSecretCollection{}
	err := c.apiClient.Ops.DoList(ManagementSecretType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ManagementSecretClient) ListAll(opts *types.ListOpts) (*ManagementSecretCollection, error) {
	resp := &ManagementSecretCollection{}
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

func (cc *ManagementSecretCollection) Next() (*ManagementSecretCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ManagementSecretCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ManagementSecretClient) ByID(id string) (*ManagementSecret, error) {
	resp := &ManagementSecret{}
	err := c.apiClient.Ops.DoByID(ManagementSecretType, id, resp)
	return resp, err
}

func (c *ManagementSecretClient) Delete(container *ManagementSecret) error {
	return c.apiClient.Ops.DoResourceDelete(ManagementSecretType, &container.Resource)
}
