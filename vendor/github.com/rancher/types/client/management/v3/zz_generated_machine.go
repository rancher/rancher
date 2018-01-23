package client

import (
	"github.com/rancher/norman/types"
)

const (
	MachineType                      = "machine"
	MachineFieldAllocatable          = "allocatable"
	MachineFieldAnnotations          = "annotations"
	MachineFieldCapacity             = "capacity"
	MachineFieldClusterId            = "clusterId"
	MachineFieldConditions           = "conditions"
	MachineFieldCreated              = "created"
	MachineFieldCreatorID            = "creatorId"
	MachineFieldCustomConfig         = "customConfig"
	MachineFieldDescription          = "description"
	MachineFieldHostname             = "hostname"
	MachineFieldIPAddress            = "ipAddress"
	MachineFieldInfo                 = "info"
	MachineFieldLabels               = "labels"
	MachineFieldLimits               = "limits"
	MachineFieldMachineTemplateId    = "machineTemplateId"
	MachineFieldName                 = "name"
	MachineFieldNamespaceId          = "namespaceId"
	MachineFieldNodeAnnotations      = "nodeAnnotations"
	MachineFieldNodeLabels           = "nodeLabels"
	MachineFieldNodeName             = "nodeName"
	MachineFieldNodeTaints           = "nodeTaints"
	MachineFieldOwnerReferences      = "ownerReferences"
	MachineFieldPodCidr              = "podCidr"
	MachineFieldProviderId           = "providerId"
	MachineFieldRemoved              = "removed"
	MachineFieldRequested            = "requested"
	MachineFieldRequestedHostname    = "requestedHostname"
	MachineFieldRole                 = "role"
	MachineFieldSSHUser              = "sshUser"
	MachineFieldState                = "state"
	MachineFieldTaints               = "taints"
	MachineFieldTransitioning        = "transitioning"
	MachineFieldTransitioningMessage = "transitioningMessage"
	MachineFieldUnschedulable        = "unschedulable"
	MachineFieldUseInternalIPAddress = "useInternalIpAddress"
	MachineFieldUuid                 = "uuid"
	MachineFieldVolumesAttached      = "volumesAttached"
	MachineFieldVolumesInUse         = "volumesInUse"
)

type Machine struct {
	types.Resource
	Allocatable          map[string]string         `json:"allocatable,omitempty"`
	Annotations          map[string]string         `json:"annotations,omitempty"`
	Capacity             map[string]string         `json:"capacity,omitempty"`
	ClusterId            string                    `json:"clusterId,omitempty"`
	Conditions           []MachineCondition        `json:"conditions,omitempty"`
	Created              string                    `json:"created,omitempty"`
	CreatorID            string                    `json:"creatorId,omitempty"`
	CustomConfig         *CustomConfig             `json:"customConfig,omitempty"`
	Description          string                    `json:"description,omitempty"`
	Hostname             string                    `json:"hostname,omitempty"`
	IPAddress            string                    `json:"ipAddress,omitempty"`
	Info                 *NodeInfo                 `json:"info,omitempty"`
	Labels               map[string]string         `json:"labels,omitempty"`
	Limits               map[string]string         `json:"limits,omitempty"`
	MachineTemplateId    string                    `json:"machineTemplateId,omitempty"`
	Name                 string                    `json:"name,omitempty"`
	NamespaceId          string                    `json:"namespaceId,omitempty"`
	NodeAnnotations      map[string]string         `json:"nodeAnnotations,omitempty"`
	NodeLabels           map[string]string         `json:"nodeLabels,omitempty"`
	NodeName             string                    `json:"nodeName,omitempty"`
	NodeTaints           []Taint                   `json:"nodeTaints,omitempty"`
	OwnerReferences      []OwnerReference          `json:"ownerReferences,omitempty"`
	PodCidr              string                    `json:"podCidr,omitempty"`
	ProviderId           string                    `json:"providerId,omitempty"`
	Removed              string                    `json:"removed,omitempty"`
	Requested            map[string]string         `json:"requested,omitempty"`
	RequestedHostname    string                    `json:"requestedHostname,omitempty"`
	Role                 []string                  `json:"role,omitempty"`
	SSHUser              string                    `json:"sshUser,omitempty"`
	State                string                    `json:"state,omitempty"`
	Taints               []Taint                   `json:"taints,omitempty"`
	Transitioning        string                    `json:"transitioning,omitempty"`
	TransitioningMessage string                    `json:"transitioningMessage,omitempty"`
	Unschedulable        *bool                     `json:"unschedulable,omitempty"`
	UseInternalIPAddress *bool                     `json:"useInternalIpAddress,omitempty"`
	Uuid                 string                    `json:"uuid,omitempty"`
	VolumesAttached      map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse         []string                  `json:"volumesInUse,omitempty"`
}
type MachineCollection struct {
	types.Collection
	Data   []Machine `json:"data,omitempty"`
	client *MachineClient
}

type MachineClient struct {
	apiClient *Client
}

type MachineOperations interface {
	List(opts *types.ListOpts) (*MachineCollection, error)
	Create(opts *Machine) (*Machine, error)
	Update(existing *Machine, updates interface{}) (*Machine, error)
	ByID(id string) (*Machine, error)
	Delete(container *Machine) error
}

func newMachineClient(apiClient *Client) *MachineClient {
	return &MachineClient{
		apiClient: apiClient,
	}
}

func (c *MachineClient) Create(container *Machine) (*Machine, error) {
	resp := &Machine{}
	err := c.apiClient.Ops.DoCreate(MachineType, container, resp)
	return resp, err
}

func (c *MachineClient) Update(existing *Machine, updates interface{}) (*Machine, error) {
	resp := &Machine{}
	err := c.apiClient.Ops.DoUpdate(MachineType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *MachineClient) List(opts *types.ListOpts) (*MachineCollection, error) {
	resp := &MachineCollection{}
	err := c.apiClient.Ops.DoList(MachineType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *MachineCollection) Next() (*MachineCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &MachineCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *MachineClient) ByID(id string) (*Machine, error) {
	resp := &Machine{}
	err := c.apiClient.Ops.DoByID(MachineType, id, resp)
	return resp, err
}

func (c *MachineClient) Delete(container *Machine) error {
	return c.apiClient.Ops.DoResourceDelete(MachineType, &container.Resource)
}
