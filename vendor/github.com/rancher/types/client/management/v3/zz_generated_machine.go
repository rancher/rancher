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
	MachineFieldConfigSource         = "configSource"
	MachineFieldCreated              = "created"
	MachineFieldDescription          = "description"
	MachineFieldDisplayName          = "displayName"
	MachineFieldExternalId           = "externalId"
	MachineFieldFinalizers           = "finalizers"
	MachineFieldHostname             = "hostname"
	MachineFieldIPAddress            = "ipAddress"
	MachineFieldId                   = "id"
	MachineFieldInfo                 = "info"
	MachineFieldLabels               = "labels"
	MachineFieldLimits               = "limits"
	MachineFieldMachineDriverConfig  = "machineDriverConfig"
	MachineFieldMachineTemplateId    = "machineTemplateId"
	MachineFieldMachineTemplateSpec  = "machineTemplateSpec"
	MachineFieldName                 = "name"
	MachineFieldNodeConfig           = "rkeNode"
	MachineFieldOwnerReferences      = "ownerReferences"
	MachineFieldPhase                = "phase"
	MachineFieldPodCIDR              = "podCIDR"
	MachineFieldProviderID           = "providerID"
	MachineFieldRemoved              = "removed"
	MachineFieldRequested            = "requested"
	MachineFieldResourcePath         = "resourcePath"
	MachineFieldRoles                = "roles"
	MachineFieldSSHUser              = "sshUser"
	MachineFieldState                = "state"
	MachineFieldTaints               = "taints"
	MachineFieldTransitioning        = "transitioning"
	MachineFieldTransitioningMessage = "transitioningMessage"
	MachineFieldUnschedulable        = "unschedulable"
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
	Conditions           []NodeCondition           `json:"conditions,omitempty"`
	ConfigSource         *NodeConfigSource         `json:"configSource,omitempty"`
	Created              string                    `json:"created,omitempty"`
	Description          string                    `json:"description,omitempty"`
	DisplayName          string                    `json:"displayName,omitempty"`
	ExternalId           string                    `json:"externalId,omitempty"`
	Finalizers           []string                  `json:"finalizers,omitempty"`
	Hostname             string                    `json:"hostname,omitempty"`
	IPAddress            string                    `json:"ipAddress,omitempty"`
	Id                   string                    `json:"id,omitempty"`
	Info                 *NodeInfo                 `json:"info,omitempty"`
	Labels               map[string]string         `json:"labels,omitempty"`
	Limits               map[string]string         `json:"limits,omitempty"`
	MachineDriverConfig  string                    `json:"machineDriverConfig,omitempty"`
	MachineTemplateId    string                    `json:"machineTemplateId,omitempty"`
	MachineTemplateSpec  *MachineTemplateSpec      `json:"machineTemplateSpec,omitempty"`
	Name                 string                    `json:"name,omitempty"`
	NodeConfig           *RKEConfigNode            `json:"rkeNode,omitempty"`
	OwnerReferences      []OwnerReference          `json:"ownerReferences,omitempty"`
	Phase                string                    `json:"phase,omitempty"`
	PodCIDR              string                    `json:"podCIDR,omitempty"`
	ProviderID           string                    `json:"providerID,omitempty"`
	Removed              string                    `json:"removed,omitempty"`
	Requested            map[string]string         `json:"requested,omitempty"`
	ResourcePath         string                    `json:"resourcePath,omitempty"`
	Roles                []string                  `json:"roles,omitempty"`
	SSHUser              string                    `json:"sshUser,omitempty"`
	State                string                    `json:"state,omitempty"`
	Taints               []Taint                   `json:"taints,omitempty"`
	Transitioning        string                    `json:"transitioning,omitempty"`
	TransitioningMessage string                    `json:"transitioningMessage,omitempty"`
	Unschedulable        *bool                     `json:"unschedulable,omitempty"`
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
