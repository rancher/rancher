package client

import (
	"github.com/rancher/norman/types"
)

const (
	FleetWorkspaceType                 = "fleetWorkspace"
	FleetWorkspaceFieldAnnotations     = "annotations"
	FleetWorkspaceFieldCreated         = "created"
	FleetWorkspaceFieldCreatorID       = "creatorId"
	FleetWorkspaceFieldLabels          = "labels"
	FleetWorkspaceFieldName            = "name"
	FleetWorkspaceFieldOwnerReferences = "ownerReferences"
	FleetWorkspaceFieldRemoved         = "removed"
	FleetWorkspaceFieldStatus          = "status"
	FleetWorkspaceFieldUUID            = "uuid"
)

type FleetWorkspace struct {
	types.Resource
	Annotations     map[string]string     `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string                `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string                `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string     `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string                `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference      `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string                `json:"removed,omitempty" yaml:"removed,omitempty"`
	Status          *FleetWorkspaceStatus `json:"status,omitempty" yaml:"status,omitempty"`
	UUID            string                `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type FleetWorkspaceCollection struct {
	types.Collection
	Data   []FleetWorkspace `json:"data,omitempty"`
	client *FleetWorkspaceClient
}

type FleetWorkspaceClient struct {
	apiClient *Client
}

type FleetWorkspaceOperations interface {
	List(opts *types.ListOpts) (*FleetWorkspaceCollection, error)
	ListAll(opts *types.ListOpts) (*FleetWorkspaceCollection, error)
	Create(opts *FleetWorkspace) (*FleetWorkspace, error)
	Update(existing *FleetWorkspace, updates interface{}) (*FleetWorkspace, error)
	Replace(existing *FleetWorkspace) (*FleetWorkspace, error)
	ByID(id string) (*FleetWorkspace, error)
	Delete(container *FleetWorkspace) error
}

func newFleetWorkspaceClient(apiClient *Client) *FleetWorkspaceClient {
	return &FleetWorkspaceClient{
		apiClient: apiClient,
	}
}

func (c *FleetWorkspaceClient) Create(container *FleetWorkspace) (*FleetWorkspace, error) {
	resp := &FleetWorkspace{}
	err := c.apiClient.Ops.DoCreate(FleetWorkspaceType, container, resp)
	return resp, err
}

func (c *FleetWorkspaceClient) Update(existing *FleetWorkspace, updates interface{}) (*FleetWorkspace, error) {
	resp := &FleetWorkspace{}
	err := c.apiClient.Ops.DoUpdate(FleetWorkspaceType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *FleetWorkspaceClient) Replace(obj *FleetWorkspace) (*FleetWorkspace, error) {
	resp := &FleetWorkspace{}
	err := c.apiClient.Ops.DoReplace(FleetWorkspaceType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *FleetWorkspaceClient) List(opts *types.ListOpts) (*FleetWorkspaceCollection, error) {
	resp := &FleetWorkspaceCollection{}
	err := c.apiClient.Ops.DoList(FleetWorkspaceType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *FleetWorkspaceClient) ListAll(opts *types.ListOpts) (*FleetWorkspaceCollection, error) {
	resp := &FleetWorkspaceCollection{}
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

func (cc *FleetWorkspaceCollection) Next() (*FleetWorkspaceCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &FleetWorkspaceCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *FleetWorkspaceClient) ByID(id string) (*FleetWorkspace, error) {
	resp := &FleetWorkspace{}
	err := c.apiClient.Ops.DoByID(FleetWorkspaceType, id, resp)
	return resp, err
}

func (c *FleetWorkspaceClient) Delete(container *FleetWorkspace) error {
	return c.apiClient.Ops.DoResourceDelete(FleetWorkspaceType, &container.Resource)
}
