package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterScanType                      = "clusterScan"
	ClusterScanFieldAnnotations          = "annotations"
	ClusterScanFieldClusterID            = "clusterId"
	ClusterScanFieldCreated              = "created"
	ClusterScanFieldCreatorID            = "creatorId"
	ClusterScanFieldLabels               = "labels"
	ClusterScanFieldManual               = "manual"
	ClusterScanFieldName                 = "name"
	ClusterScanFieldNamespaceId          = "namespaceId"
	ClusterScanFieldOwnerReferences      = "ownerReferences"
	ClusterScanFieldRemoved              = "removed"
	ClusterScanFieldScanConfig           = "scanConfig"
	ClusterScanFieldScanType             = "scanType"
	ClusterScanFieldState                = "state"
	ClusterScanFieldStatus               = "status"
	ClusterScanFieldTransitioning        = "transitioning"
	ClusterScanFieldTransitioningMessage = "transitioningMessage"
	ClusterScanFieldUUID                 = "uuid"
)

type ClusterScan struct {
	types.Resource
	Annotations          map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterID            string             `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created              string             `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string             `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels               map[string]string  `json:"labels,omitempty" yaml:"labels,omitempty"`
	Manual               bool               `json:"manual,omitempty" yaml:"manual,omitempty"`
	Name                 string             `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string             `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string             `json:"removed,omitempty" yaml:"removed,omitempty"`
	ScanConfig           *ClusterScanConfig `json:"scanConfig,omitempty" yaml:"scanConfig,omitempty"`
	ScanType             string             `json:"scanType,omitempty" yaml:"scanType,omitempty"`
	State                string             `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *ClusterScanStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ClusterScanCollection struct {
	types.Collection
	Data   []ClusterScan `json:"data,omitempty"`
	client *ClusterScanClient
}

type ClusterScanClient struct {
	apiClient *Client
}

type ClusterScanOperations interface {
	List(opts *types.ListOpts) (*ClusterScanCollection, error)
	ListAll(opts *types.ListOpts) (*ClusterScanCollection, error)
	Create(opts *ClusterScan) (*ClusterScan, error)
	Update(existing *ClusterScan, updates interface{}) (*ClusterScan, error)
	Replace(existing *ClusterScan) (*ClusterScan, error)
	ByID(id string) (*ClusterScan, error)
	Delete(container *ClusterScan) error
}

func newClusterScanClient(apiClient *Client) *ClusterScanClient {
	return &ClusterScanClient{
		apiClient: apiClient,
	}
}

func (c *ClusterScanClient) Create(container *ClusterScan) (*ClusterScan, error) {
	resp := &ClusterScan{}
	err := c.apiClient.Ops.DoCreate(ClusterScanType, container, resp)
	return resp, err
}

func (c *ClusterScanClient) Update(existing *ClusterScan, updates interface{}) (*ClusterScan, error) {
	resp := &ClusterScan{}
	err := c.apiClient.Ops.DoUpdate(ClusterScanType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterScanClient) Replace(obj *ClusterScan) (*ClusterScan, error) {
	resp := &ClusterScan{}
	err := c.apiClient.Ops.DoReplace(ClusterScanType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterScanClient) List(opts *types.ListOpts) (*ClusterScanCollection, error) {
	resp := &ClusterScanCollection{}
	err := c.apiClient.Ops.DoList(ClusterScanType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ClusterScanClient) ListAll(opts *types.ListOpts) (*ClusterScanCollection, error) {
	resp := &ClusterScanCollection{}
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

func (cc *ClusterScanCollection) Next() (*ClusterScanCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterScanCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterScanClient) ByID(id string) (*ClusterScan, error) {
	resp := &ClusterScan{}
	err := c.apiClient.Ops.DoByID(ClusterScanType, id, resp)
	return resp, err
}

func (c *ClusterScanClient) Delete(container *ClusterScan) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterScanType, &container.Resource)
}
