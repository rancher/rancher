package client

import (
	"github.com/rancher/norman/types"
)

const (
	GlobalRoleBindingType                  = "globalRoleBinding"
	GlobalRoleBindingFieldAnnotations      = "annotations"
	GlobalRoleBindingFieldCreated          = "created"
	GlobalRoleBindingFieldCreatorID        = "creatorId"
	GlobalRoleBindingFieldGlobalRoleID     = "globalRoleId"
	GlobalRoleBindingFieldGroupPrincipalID = "groupPrincipalId"
	GlobalRoleBindingFieldLabels           = "labels"
	GlobalRoleBindingFieldName             = "name"
	GlobalRoleBindingFieldOwnerReferences  = "ownerReferences"
	GlobalRoleBindingFieldRemoved          = "removed"
	GlobalRoleBindingFieldStatus           = "status"
	GlobalRoleBindingFieldUUID             = "uuid"
	GlobalRoleBindingFieldUserID           = "userId"
)

type GlobalRoleBinding struct {
	types.Resource
	Annotations      map[string]string        `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created          string                   `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID        string                   `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	GlobalRoleID     string                   `json:"globalRoleId,omitempty" yaml:"globalRoleId,omitempty"`
	GroupPrincipalID string                   `json:"groupPrincipalId,omitempty" yaml:"groupPrincipalId,omitempty"`
	Labels           map[string]string        `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name             string                   `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences  []OwnerReference         `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed          string                   `json:"removed,omitempty" yaml:"removed,omitempty"`
	Status           *GlobalRoleBindingStatus `json:"status,omitempty" yaml:"status,omitempty"`
	UUID             string                   `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserID           string                   `json:"userId,omitempty" yaml:"userId,omitempty"`
}

type GlobalRoleBindingCollection struct {
	types.Collection
	Data   []GlobalRoleBinding `json:"data,omitempty"`
	client *GlobalRoleBindingClient
}

type GlobalRoleBindingClient struct {
	apiClient *Client
}

type GlobalRoleBindingOperations interface {
	List(opts *types.ListOpts) (*GlobalRoleBindingCollection, error)
	ListAll(opts *types.ListOpts) (*GlobalRoleBindingCollection, error)
	Create(opts *GlobalRoleBinding) (*GlobalRoleBinding, error)
	Update(existing *GlobalRoleBinding, updates interface{}) (*GlobalRoleBinding, error)
	Replace(existing *GlobalRoleBinding) (*GlobalRoleBinding, error)
	ByID(id string) (*GlobalRoleBinding, error)
	Delete(container *GlobalRoleBinding) error
}

func newGlobalRoleBindingClient(apiClient *Client) *GlobalRoleBindingClient {
	return &GlobalRoleBindingClient{
		apiClient: apiClient,
	}
}

func (c *GlobalRoleBindingClient) Create(container *GlobalRoleBinding) (*GlobalRoleBinding, error) {
	resp := &GlobalRoleBinding{}
	err := c.apiClient.Ops.DoCreate(GlobalRoleBindingType, container, resp)
	return resp, err
}

func (c *GlobalRoleBindingClient) Update(existing *GlobalRoleBinding, updates interface{}) (*GlobalRoleBinding, error) {
	resp := &GlobalRoleBinding{}
	err := c.apiClient.Ops.DoUpdate(GlobalRoleBindingType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GlobalRoleBindingClient) Replace(obj *GlobalRoleBinding) (*GlobalRoleBinding, error) {
	resp := &GlobalRoleBinding{}
	err := c.apiClient.Ops.DoReplace(GlobalRoleBindingType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *GlobalRoleBindingClient) List(opts *types.ListOpts) (*GlobalRoleBindingCollection, error) {
	resp := &GlobalRoleBindingCollection{}
	err := c.apiClient.Ops.DoList(GlobalRoleBindingType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *GlobalRoleBindingClient) ListAll(opts *types.ListOpts) (*GlobalRoleBindingCollection, error) {
	resp := &GlobalRoleBindingCollection{}
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

func (cc *GlobalRoleBindingCollection) Next() (*GlobalRoleBindingCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GlobalRoleBindingCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GlobalRoleBindingClient) ByID(id string) (*GlobalRoleBinding, error) {
	resp := &GlobalRoleBinding{}
	err := c.apiClient.Ops.DoByID(GlobalRoleBindingType, id, resp)
	return resp, err
}

func (c *GlobalRoleBindingClient) Delete(container *GlobalRoleBinding) error {
	return c.apiClient.Ops.DoResourceDelete(GlobalRoleBindingType, &container.Resource)
}
