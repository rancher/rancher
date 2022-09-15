package client

import (
	"github.com/rancher/norman/types"
)

const (
	MachineType            = "cluster.x-k8s.io.machine"
	MachineFieldCreatorID  = "creatorId"
	MachineFieldObjectMeta = "metadata"
	MachineFieldSpec       = "spec"
	MachineFieldStatus     = "status"
)

type Machine struct {
	types.Resource
	CreatorID  string         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ObjectMeta *ObjectMeta    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       *MachineSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status     *MachineStatus `json:"status,omitempty" yaml:"status,omitempty"`
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
