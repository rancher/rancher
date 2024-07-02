package client

import (
	"github.com/rancher/norman/types"
)

const (
	NodeType                      = "node"
	NodeFieldAllocatable          = "allocatable"
	NodeFieldAnnotations          = "annotations"
	NodeFieldAppliedNodeVersion   = "appliedNodeVersion"
	NodeFieldCapacity             = "capacity"
	NodeFieldClusterID            = "clusterId"
	NodeFieldConditions           = "conditions"
	NodeFieldControlPlane         = "controlPlane"
	NodeFieldCreated              = "created"
	NodeFieldCreatorID            = "creatorId"
	NodeFieldCustomConfig         = "customConfig"
	NodeFieldDescription          = "description"
	NodeFieldDockerInfo           = "dockerInfo"
	NodeFieldEtcd                 = "etcd"
	NodeFieldExternalIPAddress    = "externalIpAddress"
	NodeFieldHostname             = "hostname"
	NodeFieldIPAddress            = "ipAddress"
	NodeFieldImported             = "imported"
	NodeFieldInfo                 = "info"
	NodeFieldLabels               = "labels"
	NodeFieldLimits               = "limits"
	NodeFieldName                 = "name"
	NodeFieldNamespaceId          = "namespaceId"
	NodeFieldNodeName             = "nodeName"
	NodeFieldNodePlan             = "nodePlan"
	NodeFieldNodePoolID           = "nodePoolId"
	NodeFieldNodeTaints           = "nodeTaints"
	NodeFieldNodeTemplateID       = "nodeTemplateId"
	NodeFieldOwnerReferences      = "ownerReferences"
	NodeFieldPodCidr              = "podCidr"
	NodeFieldPodCidrs             = "podCidrs"
	NodeFieldProviderId           = "providerId"
	NodeFieldPublicEndpoints      = "publicEndpoints"
	NodeFieldRemoved              = "removed"
	NodeFieldRequested            = "requested"
	NodeFieldRequestedHostname    = "requestedHostname"
	NodeFieldRuntimeHandlers      = "runtimeHandlers"
	NodeFieldScaledownTime        = "scaledownTime"
	NodeFieldSshUser              = "sshUser"
	NodeFieldState                = "state"
	NodeFieldTaints               = "taints"
	NodeFieldTransitioning        = "transitioning"
	NodeFieldTransitioningMessage = "transitioningMessage"
	NodeFieldUUID                 = "uuid"
	NodeFieldUnschedulable        = "unschedulable"
	NodeFieldVolumesAttached      = "volumesAttached"
	NodeFieldVolumesInUse         = "volumesInUse"
	NodeFieldWorker               = "worker"
)

type Node struct {
	types.Resource
	Allocatable          map[string]string         `json:"allocatable,omitempty" yaml:"allocatable,omitempty"`
	Annotations          map[string]string         `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AppliedNodeVersion   int64                     `json:"appliedNodeVersion,omitempty" yaml:"appliedNodeVersion,omitempty"`
	Capacity             map[string]string         `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	ClusterID            string                    `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Conditions           []NodeCondition           `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	ControlPlane         bool                      `json:"controlPlane,omitempty" yaml:"controlPlane,omitempty"`
	Created              string                    `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string                    `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	CustomConfig         *CustomConfig             `json:"customConfig,omitempty" yaml:"customConfig,omitempty"`
	Description          string                    `json:"description,omitempty" yaml:"description,omitempty"`
	DockerInfo           *DockerInfo               `json:"dockerInfo,omitempty" yaml:"dockerInfo,omitempty"`
	Etcd                 bool                      `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	ExternalIPAddress    string                    `json:"externalIpAddress,omitempty" yaml:"externalIpAddress,omitempty"`
	Hostname             string                    `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IPAddress            string                    `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`
	Imported             bool                      `json:"imported,omitempty" yaml:"imported,omitempty"`
	Info                 *NodeInfo                 `json:"info,omitempty" yaml:"info,omitempty"`
	Labels               map[string]string         `json:"labels,omitempty" yaml:"labels,omitempty"`
	Limits               map[string]string         `json:"limits,omitempty" yaml:"limits,omitempty"`
	Name                 string                    `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string                    `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeName             string                    `json:"nodeName,omitempty" yaml:"nodeName,omitempty"`
	NodePlan             *NodePlan                 `json:"nodePlan,omitempty" yaml:"nodePlan,omitempty"`
	NodePoolID           string                    `json:"nodePoolId,omitempty" yaml:"nodePoolId,omitempty"`
	NodeTaints           []Taint                   `json:"nodeTaints,omitempty" yaml:"nodeTaints,omitempty"`
	NodeTemplateID       string                    `json:"nodeTemplateId,omitempty" yaml:"nodeTemplateId,omitempty"`
	OwnerReferences      []OwnerReference          `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PodCidr              string                    `json:"podCidr,omitempty" yaml:"podCidr,omitempty"`
	PodCidrs             []string                  `json:"podCidrs,omitempty" yaml:"podCidrs,omitempty"`
	ProviderId           string                    `json:"providerId,omitempty" yaml:"providerId,omitempty"`
	PublicEndpoints      []PublicEndpoint          `json:"publicEndpoints,omitempty" yaml:"publicEndpoints,omitempty"`
	Removed              string                    `json:"removed,omitempty" yaml:"removed,omitempty"`
	Requested            map[string]string         `json:"requested,omitempty" yaml:"requested,omitempty"`
	RequestedHostname    string                    `json:"requestedHostname,omitempty" yaml:"requestedHostname,omitempty"`
	RuntimeHandlers      []NodeRuntimeHandler      `json:"runtimeHandlers,omitempty" yaml:"runtimeHandlers,omitempty"`
	ScaledownTime        string                    `json:"scaledownTime,omitempty" yaml:"scaledownTime,omitempty"`
	SshUser              string                    `json:"sshUser,omitempty" yaml:"sshUser,omitempty"`
	State                string                    `json:"state,omitempty" yaml:"state,omitempty"`
	Taints               []Taint                   `json:"taints,omitempty" yaml:"taints,omitempty"`
	Transitioning        string                    `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string                    `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string                    `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Unschedulable        bool                      `json:"unschedulable,omitempty" yaml:"unschedulable,omitempty"`
	VolumesAttached      map[string]AttachedVolume `json:"volumesAttached,omitempty" yaml:"volumesAttached,omitempty"`
	VolumesInUse         []string                  `json:"volumesInUse,omitempty" yaml:"volumesInUse,omitempty"`
	Worker               bool                      `json:"worker,omitempty" yaml:"worker,omitempty"`
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
	ListAll(opts *types.ListOpts) (*NodeCollection, error)
	Create(opts *Node) (*Node, error)
	Update(existing *Node, updates interface{}) (*Node, error)
	Replace(existing *Node) (*Node, error)
	ByID(id string) (*Node, error)
	Delete(container *Node) error

	ActionCordon(resource *Node) error

	ActionDrain(resource *Node, input *NodeDrainInput) error

	ActionScaledown(resource *Node) error

	ActionStopDrain(resource *Node) error

	ActionUncordon(resource *Node) error
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

func (c *NodeClient) Replace(obj *Node) (*Node, error) {
	resp := &Node{}
	err := c.apiClient.Ops.DoReplace(NodeType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *NodeClient) List(opts *types.ListOpts) (*NodeCollection, error) {
	resp := &NodeCollection{}
	err := c.apiClient.Ops.DoList(NodeType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *NodeClient) ListAll(opts *types.ListOpts) (*NodeCollection, error) {
	resp := &NodeCollection{}
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

func (c *NodeClient) ActionCordon(resource *Node) error {
	err := c.apiClient.Ops.DoAction(NodeType, "cordon", &resource.Resource, nil, nil)
	return err
}

func (c *NodeClient) ActionDrain(resource *Node, input *NodeDrainInput) error {
	err := c.apiClient.Ops.DoAction(NodeType, "drain", &resource.Resource, input, nil)
	return err
}

func (c *NodeClient) ActionScaledown(resource *Node) error {
	err := c.apiClient.Ops.DoAction(NodeType, "scaledown", &resource.Resource, nil, nil)
	return err
}

func (c *NodeClient) ActionStopDrain(resource *Node) error {
	err := c.apiClient.Ops.DoAction(NodeType, "stopDrain", &resource.Resource, nil, nil)
	return err
}

func (c *NodeClient) ActionUncordon(resource *Node) error {
	err := c.apiClient.Ops.DoAction(NodeType, "uncordon", &resource.Resource, nil, nil)
	return err
}
