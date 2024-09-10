package client

import (
	"github.com/rancher/norman/types"
)

const (
	MachineType                         = "cluster.x-k8s.io.machine"
	MachineFieldAnnotations             = "annotations"
	MachineFieldBootstrap               = "bootstrap"
	MachineFieldClusterName             = "clusterName"
	MachineFieldCreated                 = "created"
	MachineFieldCreatorID               = "creatorId"
	MachineFieldFailureDomain           = "failureDomain"
	MachineFieldInfrastructureRef       = "infrastructureRef"
	MachineFieldLabels                  = "labels"
	MachineFieldName                    = "name"
	MachineFieldNodeDeletionTimeout     = "nodeDeletionTimeout"
	MachineFieldNodeDrainTimeout        = "nodeDrainTimeout"
	MachineFieldNodeVolumeDetachTimeout = "nodeVolumeDetachTimeout"
	MachineFieldOwnerReferences         = "ownerReferences"
	MachineFieldProviderID              = "providerID"
	MachineFieldRemoved                 = "removed"
	MachineFieldState                   = "state"
	MachineFieldStatus                  = "status"
	MachineFieldTransitioning           = "transitioning"
	MachineFieldTransitioningMessage    = "transitioningMessage"
	MachineFieldUUID                    = "uuid"
	MachineFieldVersion                 = "version"
)

type Machine struct {
	types.Resource
	Annotations             map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Bootstrap               *Bootstrap        `json:"bootstrap,omitempty" yaml:"bootstrap,omitempty"`
	ClusterName             string            `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	Created                 string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID               string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	FailureDomain           string            `json:"failureDomain,omitempty" yaml:"failureDomain,omitempty"`
	InfrastructureRef       *ObjectReference  `json:"infrastructureRef,omitempty" yaml:"infrastructureRef,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                    string            `json:"name,omitempty" yaml:"name,omitempty"`
	NodeDeletionTimeout     *Duration         `json:"nodeDeletionTimeout,omitempty" yaml:"nodeDeletionTimeout,omitempty"`
	NodeDrainTimeout        *Duration         `json:"nodeDrainTimeout,omitempty" yaml:"nodeDrainTimeout,omitempty"`
	NodeVolumeDetachTimeout *Duration         `json:"nodeVolumeDetachTimeout,omitempty" yaml:"nodeVolumeDetachTimeout,omitempty"`
	OwnerReferences         []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProviderID              string            `json:"providerID,omitempty" yaml:"providerID,omitempty"`
	Removed                 string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                   string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status                  *MachineStatus    `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning           string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage    string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                    string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Version                 string            `json:"version,omitempty" yaml:"version,omitempty"`
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
	ListAll(opts *types.ListOpts) (*MachineCollection, error)
	Create(opts *Machine) (*Machine, error)
	Update(existing *Machine, updates interface{}) (*Machine, error)
	Replace(existing *Machine) (*Machine, error)
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

func (c *MachineClient) Replace(obj *Machine) (*Machine, error) {
	resp := &Machine{}
	err := c.apiClient.Ops.DoReplace(MachineType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *MachineClient) List(opts *types.ListOpts) (*MachineCollection, error) {
	resp := &MachineCollection{}
	err := c.apiClient.Ops.DoList(MachineType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *MachineClient) ListAll(opts *types.ListOpts) (*MachineCollection, error) {
	resp := &MachineCollection{}
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
