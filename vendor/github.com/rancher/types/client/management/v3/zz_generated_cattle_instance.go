package client

import (
	"github.com/rancher/norman/types"
)

const (
	CattleInstanceType                 = "cattleInstance"
	CattleInstanceFieldAnnotations     = "annotations"
	CattleInstanceFieldCreated         = "created"
	CattleInstanceFieldCreatorID       = "creatorId"
	CattleInstanceFieldIdentity        = "identity"
	CattleInstanceFieldLabels          = "labels"
	CattleInstanceFieldName            = "name"
	CattleInstanceFieldOwnerReferences = "ownerReferences"
	CattleInstanceFieldRemoved         = "removed"
	CattleInstanceFieldUUID            = "uuid"
)

type CattleInstance struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Identity        string            `json:"identity,omitempty" yaml:"identity,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type CattleInstanceCollection struct {
	types.Collection
	Data   []CattleInstance `json:"data,omitempty"`
	client *CattleInstanceClient
}

type CattleInstanceClient struct {
	apiClient *Client
}

type CattleInstanceOperations interface {
	List(opts *types.ListOpts) (*CattleInstanceCollection, error)
	Create(opts *CattleInstance) (*CattleInstance, error)
	Update(existing *CattleInstance, updates interface{}) (*CattleInstance, error)
	Replace(existing *CattleInstance) (*CattleInstance, error)
	ByID(id string) (*CattleInstance, error)
	Delete(container *CattleInstance) error
}

func newCattleInstanceClient(apiClient *Client) *CattleInstanceClient {
	return &CattleInstanceClient{
		apiClient: apiClient,
	}
}

func (c *CattleInstanceClient) Create(container *CattleInstance) (*CattleInstance, error) {
	resp := &CattleInstance{}
	err := c.apiClient.Ops.DoCreate(CattleInstanceType, container, resp)
	return resp, err
}

func (c *CattleInstanceClient) Update(existing *CattleInstance, updates interface{}) (*CattleInstance, error) {
	resp := &CattleInstance{}
	err := c.apiClient.Ops.DoUpdate(CattleInstanceType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CattleInstanceClient) Replace(obj *CattleInstance) (*CattleInstance, error) {
	resp := &CattleInstance{}
	err := c.apiClient.Ops.DoReplace(CattleInstanceType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *CattleInstanceClient) List(opts *types.ListOpts) (*CattleInstanceCollection, error) {
	resp := &CattleInstanceCollection{}
	err := c.apiClient.Ops.DoList(CattleInstanceType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *CattleInstanceCollection) Next() (*CattleInstanceCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CattleInstanceCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CattleInstanceClient) ByID(id string) (*CattleInstance, error) {
	resp := &CattleInstance{}
	err := c.apiClient.Ops.DoByID(CattleInstanceType, id, resp)
	return resp, err
}

func (c *CattleInstanceClient) Delete(container *CattleInstance) error {
	return c.apiClient.Ops.DoResourceDelete(CattleInstanceType, &container.Resource)
}
