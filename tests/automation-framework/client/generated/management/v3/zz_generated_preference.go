package client

import (
	"github.com/rancher/norman/types"
)

const (
	PreferenceType                 = "preference"
	PreferenceFieldAnnotations     = "annotations"
	PreferenceFieldCreated         = "created"
	PreferenceFieldCreatorID       = "creatorId"
	PreferenceFieldLabels          = "labels"
	PreferenceFieldName            = "name"
	PreferenceFieldNamespaceId     = "namespaceId"
	PreferenceFieldOwnerReferences = "ownerReferences"
	PreferenceFieldRemoved         = "removed"
	PreferenceFieldUUID            = "uuid"
	PreferenceFieldValue           = "value"
)

type Preference struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Value           string            `json:"value,omitempty" yaml:"value,omitempty"`
}

type PreferenceCollection struct {
	types.Collection
	Data   []Preference `json:"data,omitempty"`
	client *PreferenceClient
}

type PreferenceClient struct {
	apiClient *Client
}

type PreferenceOperations interface {
	List(opts *types.ListOpts) (*PreferenceCollection, error)
	ListAll(opts *types.ListOpts) (*PreferenceCollection, error)
	Create(opts *Preference) (*Preference, error)
	Update(existing *Preference, updates interface{}) (*Preference, error)
	Replace(existing *Preference) (*Preference, error)
	ByID(id string) (*Preference, error)
	Delete(container *Preference) error
}

func newPreferenceClient(apiClient *Client) *PreferenceClient {
	return &PreferenceClient{
		apiClient: apiClient,
	}
}

func (c *PreferenceClient) Create(container *Preference) (*Preference, error) {
	resp := &Preference{}
	err := c.apiClient.Ops.DoCreate(PreferenceType, container, resp)
	return resp, err
}

func (c *PreferenceClient) Update(existing *Preference, updates interface{}) (*Preference, error) {
	resp := &Preference{}
	err := c.apiClient.Ops.DoUpdate(PreferenceType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PreferenceClient) Replace(obj *Preference) (*Preference, error) {
	resp := &Preference{}
	err := c.apiClient.Ops.DoReplace(PreferenceType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PreferenceClient) List(opts *types.ListOpts) (*PreferenceCollection, error) {
	resp := &PreferenceCollection{}
	err := c.apiClient.Ops.DoList(PreferenceType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *PreferenceClient) ListAll(opts *types.ListOpts) (*PreferenceCollection, error) {
	resp := &PreferenceCollection{}
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

func (cc *PreferenceCollection) Next() (*PreferenceCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PreferenceCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PreferenceClient) ByID(id string) (*Preference, error) {
	resp := &Preference{}
	err := c.apiClient.Ops.DoByID(PreferenceType, id, resp)
	return resp, err
}

func (c *PreferenceClient) Delete(container *Preference) error {
	return c.apiClient.Ops.DoResourceDelete(PreferenceType, &container.Resource)
}
