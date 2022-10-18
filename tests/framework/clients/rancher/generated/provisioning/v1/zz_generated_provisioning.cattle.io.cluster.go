package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterType            = "provisioning.cattle.io.cluster"
	ClusterFieldCreatorID  = "creatorId"
	ClusterFieldObjectMeta = "metadata"
	ClusterFieldSpec       = "spec"
	ClusterFieldStatus     = "status"
)

type Cluster struct {
	types.Resource
	CreatorID  string         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ObjectMeta *ObjectMeta    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       *ClusterSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status     *ClusterStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type ClusterCollection struct {
	types.Collection
	Data   []Cluster `json:"data,omitempty"`
	client *ClusterClient
}

type ClusterClient struct {
	apiClient *Client
}

type ClusterOperations interface {
	List(opts *types.ListOpts) (*ClusterCollection, error)
	ListAll(opts *types.ListOpts) (*ClusterCollection, error)
	Create(opts *Cluster) (*Cluster, error)
	Update(existing *Cluster, updates interface{}) (*Cluster, error)
	Replace(existing *Cluster) (*Cluster, error)
	ByID(id string) (*Cluster, error)
	Delete(container *Cluster) error
}

func newClusterClient(apiClient *Client) *ClusterClient {
	return &ClusterClient{
		apiClient: apiClient,
	}
}

func (c *ClusterClient) Create(container *Cluster) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoCreate(ClusterType, container, resp)
	return resp, err
}

func (c *ClusterClient) Update(existing *Cluster, updates interface{}) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoUpdate(ClusterType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterClient) Replace(obj *Cluster) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoReplace(ClusterType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterClient) List(opts *types.ListOpts) (*ClusterCollection, error) {
	resp := &ClusterCollection{}
	err := c.apiClient.Ops.DoList(ClusterType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ClusterClient) ListAll(opts *types.ListOpts) (*ClusterCollection, error) {
	resp := &ClusterCollection{}
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

func (cc *ClusterCollection) Next() (*ClusterCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterClient) ByID(id string) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoByID(ClusterType, id, resp)
	return resp, err
}

func (c *ClusterClient) Delete(container *Cluster) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterType, &container.Resource)
}
