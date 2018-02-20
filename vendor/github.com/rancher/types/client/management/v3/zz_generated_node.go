package client

import (
	"github.com/rancher/norman/types"
)

const (
	NodeType                      = "node"
	NodeFieldAllocatable          = "allocatable"
	NodeFieldAnnotations          = "annotations"
	NodeFieldCapacity             = "capacity"
	NodeFieldClusterId            = "clusterId"
	NodeFieldConditions           = "conditions"
	NodeFieldControlPlane         = "controlPlane"
	NodeFieldCreated              = "created"
	NodeFieldCreatorID            = "creatorId"
	NodeFieldCustomConfig         = "customConfig"
	NodeFieldDescription          = "description"
	NodeFieldEtcd                 = "etcd"
	NodeFieldHostname             = "hostname"
	NodeFieldIPAddress            = "ipAddress"
	NodeFieldImported             = "imported"
	NodeFieldInfo                 = "info"
	NodeFieldLabels               = "labels"
	NodeFieldLimits               = "limits"
	NodeFieldName                 = "name"
	NodeFieldNamespaceId          = "namespaceId"
	NodeFieldNodeName             = "nodeName"
	NodeFieldNodePoolName         = "nodePoolUuid"
	NodeFieldNodeTaints           = "nodeTaints"
	NodeFieldNodeTemplateId       = "nodeTemplateId"
	NodeFieldOwnerReferences      = "ownerReferences"
	NodeFieldPodCidr              = "podCidr"
	NodeFieldProviderId           = "providerId"
	NodeFieldRemoved              = "removed"
	NodeFieldRequested            = "requested"
	NodeFieldRequestedHostname    = "requestedHostname"
	NodeFieldSshUser              = "sshUser"
	NodeFieldState                = "state"
	NodeFieldTaints               = "taints"
	NodeFieldTransitioning        = "transitioning"
	NodeFieldTransitioningMessage = "transitioningMessage"
	NodeFieldUnschedulable        = "unschedulable"
	NodeFieldUuid                 = "uuid"
	NodeFieldVolumesAttached      = "volumesAttached"
	NodeFieldVolumesInUse         = "volumesInUse"
	NodeFieldWorker               = "worker"
)

type Node struct {
	types.Resource
	Allocatable          map[string]string         `json:"allocatable,omitempty"`
	Annotations          map[string]string         `json:"annotations,omitempty"`
	Capacity             map[string]string         `json:"capacity,omitempty"`
	ClusterId            string                    `json:"clusterId,omitempty"`
	Conditions           []NodeCondition           `json:"conditions,omitempty"`
	ControlPlane         bool                      `json:"controlPlane,omitempty"`
	Created              string                    `json:"created,omitempty"`
	CreatorID            string                    `json:"creatorId,omitempty"`
	CustomConfig         *CustomConfig             `json:"customConfig,omitempty"`
	Description          string                    `json:"description,omitempty"`
	Etcd                 bool                      `json:"etcd,omitempty"`
	Hostname             string                    `json:"hostname,omitempty"`
	IPAddress            string                    `json:"ipAddress,omitempty"`
	Imported             bool                      `json:"imported,omitempty"`
	Info                 *NodeInfo                 `json:"info,omitempty"`
	Labels               map[string]string         `json:"labels,omitempty"`
	Limits               map[string]string         `json:"limits,omitempty"`
	Name                 string                    `json:"name,omitempty"`
	NamespaceId          string                    `json:"namespaceId,omitempty"`
	NodeName             string                    `json:"nodeName,omitempty"`
	NodePoolName         string                    `json:"nodePoolUuid,omitempty"`
	NodeTaints           []Taint                   `json:"nodeTaints,omitempty"`
	NodeTemplateId       string                    `json:"nodeTemplateId,omitempty"`
	OwnerReferences      []OwnerReference          `json:"ownerReferences,omitempty"`
	PodCidr              string                    `json:"podCidr,omitempty"`
	ProviderId           string                    `json:"providerId,omitempty"`
	Removed              string                    `json:"removed,omitempty"`
	Requested            map[string]string         `json:"requested,omitempty"`
	RequestedHostname    string                    `json:"requestedHostname,omitempty"`
	SshUser              string                    `json:"sshUser,omitempty"`
	State                string                    `json:"state,omitempty"`
	Taints               []Taint                   `json:"taints,omitempty"`
	Transitioning        string                    `json:"transitioning,omitempty"`
	TransitioningMessage string                    `json:"transitioningMessage,omitempty"`
	Unschedulable        bool                      `json:"unschedulable,omitempty"`
	Uuid                 string                    `json:"uuid,omitempty"`
	VolumesAttached      map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse         []string                  `json:"volumesInUse,omitempty"`
	Worker               bool                      `json:"worker,omitempty"`
}
type NodeCollection struct {
	types.Collection
	Data   []Node `json:"data,omitempty"`
	client *NodeClient
}

type NodeClient struct {
	apiClient *Client
}

type NodeOperations interface {
	List(opts *types.ListOpts) (*NodeCollection, error)
	Create(opts *Node) (*Node, error)
	Update(existing *Node, updates interface{}) (*Node, error)
	ByID(id string) (*Node, error)
	Delete(container *Node) error
}

func newNodeClient(apiClient *Client) *NodeClient {
	return &NodeClient{
		apiClient: apiClient,
	}
}

func (c *NodeClient) Create(container *Node) (*Node, error) {
	resp := &Node{}
	err := c.apiClient.Ops.DoCreate(NodeType, container, resp)
	return resp, err
}

func (c *NodeClient) Update(existing *Node, updates interface{}) (*Node, error) {
	resp := &Node{}
	err := c.apiClient.Ops.DoUpdate(NodeType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NodeClient) List(opts *types.ListOpts) (*NodeCollection, error) {
	resp := &NodeCollection{}
	err := c.apiClient.Ops.DoList(NodeType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *NodeCollection) Next() (*NodeCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NodeCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NodeClient) ByID(id string) (*Node, error) {
	resp := &Node{}
	err := c.apiClient.Ops.DoByID(NodeType, id, resp)
	return resp, err
}

func (c *NodeClient) Delete(container *Node) error {
	return c.apiClient.Ops.DoResourceDelete(NodeType, &container.Resource)
}
