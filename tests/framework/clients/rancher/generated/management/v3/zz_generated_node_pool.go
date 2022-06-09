package client

import (
	"github.com/rancher/norman/types"
)

const (
	NodePoolType                         = "nodePool"
	NodePoolFieldAnnotations             = "annotations"
	NodePoolFieldClusterID               = "clusterId"
	NodePoolFieldControlPlane            = "controlPlane"
	NodePoolFieldCreated                 = "created"
	NodePoolFieldCreatorID               = "creatorId"
	NodePoolFieldDeleteNotReadyAfterSecs = "deleteNotReadyAfterSecs"
	NodePoolFieldDisplayName             = "displayName"
	NodePoolFieldDrainBeforeDelete       = "drainBeforeDelete"
	NodePoolFieldDriver                  = "driver"
	NodePoolFieldEtcd                    = "etcd"
	NodePoolFieldHostnamePrefix          = "hostnamePrefix"
	NodePoolFieldLabels                  = "labels"
	NodePoolFieldName                    = "name"
	NodePoolFieldNamespaceId             = "namespaceId"
	NodePoolFieldNodeAnnotations         = "nodeAnnotations"
	NodePoolFieldNodeLabels              = "nodeLabels"
	NodePoolFieldNodeTaints              = "nodeTaints"
	NodePoolFieldNodeTemplateID          = "nodeTemplateId"
	NodePoolFieldOwnerReferences         = "ownerReferences"
	NodePoolFieldQuantity                = "quantity"
	NodePoolFieldRemoved                 = "removed"
	NodePoolFieldState                   = "state"
	NodePoolFieldStatus                  = "status"
	NodePoolFieldTransitioning           = "transitioning"
	NodePoolFieldTransitioningMessage    = "transitioningMessage"
	NodePoolFieldUUID                    = "uuid"
	NodePoolFieldWorker                  = "worker"
)

type NodePool struct {
	types.Resource
	Annotations             map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterID               string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ControlPlane            bool              `json:"controlPlane,omitempty" yaml:"controlPlane,omitempty"`
	Created                 string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID               string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DeleteNotReadyAfterSecs int64             `json:"deleteNotReadyAfterSecs,omitempty" yaml:"deleteNotReadyAfterSecs,omitempty"`
	DisplayName             string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	DrainBeforeDelete       bool              `json:"drainBeforeDelete,omitempty" yaml:"drainBeforeDelete,omitempty"`
	Driver                  string            `json:"driver,omitempty" yaml:"driver,omitempty"`
	Etcd                    bool              `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	HostnamePrefix          string            `json:"hostnamePrefix,omitempty" yaml:"hostnamePrefix,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                    string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId             string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeAnnotations         map[string]string `json:"nodeAnnotations,omitempty" yaml:"nodeAnnotations,omitempty"`
	NodeLabels              map[string]string `json:"nodeLabels,omitempty" yaml:"nodeLabels,omitempty"`
	NodeTaints              []Taint           `json:"nodeTaints,omitempty" yaml:"nodeTaints,omitempty"`
	NodeTemplateID          string            `json:"nodeTemplateId,omitempty" yaml:"nodeTemplateId,omitempty"`
	OwnerReferences         []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Quantity                int64             `json:"quantity,omitempty" yaml:"quantity,omitempty"`
	Removed                 string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                   string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status                  *NodePoolStatus   `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning           string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage    string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                    string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Worker                  bool              `json:"worker,omitempty" yaml:"worker,omitempty"`
}

type NodePoolCollection struct {
	types.Collection
	Data   []NodePool `json:"data,omitempty"`
	client *NodePoolClient
}

type NodePoolClient struct {
	apiClient *Client
}

type NodePoolOperations interface {
	List(opts *types.ListOpts) (*NodePoolCollection, error)
	ListAll(opts *types.ListOpts) (*NodePoolCollection, error)
	Create(opts *NodePool) (*NodePool, error)
	Update(existing *NodePool, updates interface{}) (*NodePool, error)
	Replace(existing *NodePool) (*NodePool, error)
	ByID(id string) (*NodePool, error)
	Delete(container *NodePool) error
}

func newNodePoolClient(apiClient *Client) *NodePoolClient {
	return &NodePoolClient{
		apiClient: apiClient,
	}
}

func (c *NodePoolClient) Create(container *NodePool) (*NodePool, error) {
	resp := &NodePool{}
	err := c.apiClient.Ops.DoCreate(NodePoolType, container, resp)
	return resp, err
}

func (c *NodePoolClient) Update(existing *NodePool, updates interface{}) (*NodePool, error) {
	resp := &NodePool{}
	err := c.apiClient.Ops.DoUpdate(NodePoolType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NodePoolClient) Replace(obj *NodePool) (*NodePool, error) {
	resp := &NodePool{}
	err := c.apiClient.Ops.DoReplace(NodePoolType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *NodePoolClient) List(opts *types.ListOpts) (*NodePoolCollection, error) {
	resp := &NodePoolCollection{}
	err := c.apiClient.Ops.DoList(NodePoolType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *NodePoolClient) ListAll(opts *types.ListOpts) (*NodePoolCollection, error) {
	resp := &NodePoolCollection{}
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

func (cc *NodePoolCollection) Next() (*NodePoolCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NodePoolCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NodePoolClient) ByID(id string) (*NodePool, error) {
	resp := &NodePool{}
	err := c.apiClient.Ops.DoByID(NodePoolType, id, resp)
	return resp, err
}

func (c *NodePoolClient) Delete(container *NodePool) error {
	return c.apiClient.Ops.DoResourceDelete(NodePoolType, &container.Resource)
}
