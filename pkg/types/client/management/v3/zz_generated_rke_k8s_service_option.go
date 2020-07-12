package client

import (
	"github.com/rancher/norman/types"
)

const (
	RkeK8sServiceOptionType                 = "rkeK8sServiceOption"
	RkeK8sServiceOptionFieldAnnotations     = "annotations"
	RkeK8sServiceOptionFieldCreated         = "created"
	RkeK8sServiceOptionFieldCreatorID       = "creatorId"
	RkeK8sServiceOptionFieldLabels          = "labels"
	RkeK8sServiceOptionFieldName            = "name"
	RkeK8sServiceOptionFieldOwnerReferences = "ownerReferences"
	RkeK8sServiceOptionFieldRemoved         = "removed"
	RkeK8sServiceOptionFieldServiceOptions  = "serviceOptions"
	RkeK8sServiceOptionFieldUUID            = "uuid"
)

type RkeK8sServiceOption struct {
	types.Resource
	Annotations     map[string]string          `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string                     `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string                     `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string          `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string                     `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference           `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string                     `json:"removed,omitempty" yaml:"removed,omitempty"`
	ServiceOptions  *KubernetesServicesOptions `json:"serviceOptions,omitempty" yaml:"serviceOptions,omitempty"`
	UUID            string                     `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type RkeK8sServiceOptionCollection struct {
	types.Collection
	Data   []RkeK8sServiceOption `json:"data,omitempty"`
	client *RkeK8sServiceOptionClient
}

type RkeK8sServiceOptionClient struct {
	apiClient *Client
}

type RkeK8sServiceOptionOperations interface {
	List(opts *types.ListOpts) (*RkeK8sServiceOptionCollection, error)
	ListAll(opts *types.ListOpts) (*RkeK8sServiceOptionCollection, error)
	Create(opts *RkeK8sServiceOption) (*RkeK8sServiceOption, error)
	Update(existing *RkeK8sServiceOption, updates interface{}) (*RkeK8sServiceOption, error)
	Replace(existing *RkeK8sServiceOption) (*RkeK8sServiceOption, error)
	ByID(id string) (*RkeK8sServiceOption, error)
	Delete(container *RkeK8sServiceOption) error
}

func newRkeK8sServiceOptionClient(apiClient *Client) *RkeK8sServiceOptionClient {
	return &RkeK8sServiceOptionClient{
		apiClient: apiClient,
	}
}

func (c *RkeK8sServiceOptionClient) Create(container *RkeK8sServiceOption) (*RkeK8sServiceOption, error) {
	resp := &RkeK8sServiceOption{}
	err := c.apiClient.Ops.DoCreate(RkeK8sServiceOptionType, container, resp)
	return resp, err
}

func (c *RkeK8sServiceOptionClient) Update(existing *RkeK8sServiceOption, updates interface{}) (*RkeK8sServiceOption, error) {
	resp := &RkeK8sServiceOption{}
	err := c.apiClient.Ops.DoUpdate(RkeK8sServiceOptionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RkeK8sServiceOptionClient) Replace(obj *RkeK8sServiceOption) (*RkeK8sServiceOption, error) {
	resp := &RkeK8sServiceOption{}
	err := c.apiClient.Ops.DoReplace(RkeK8sServiceOptionType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RkeK8sServiceOptionClient) List(opts *types.ListOpts) (*RkeK8sServiceOptionCollection, error) {
	resp := &RkeK8sServiceOptionCollection{}
	err := c.apiClient.Ops.DoList(RkeK8sServiceOptionType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *RkeK8sServiceOptionClient) ListAll(opts *types.ListOpts) (*RkeK8sServiceOptionCollection, error) {
	resp := &RkeK8sServiceOptionCollection{}
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

func (cc *RkeK8sServiceOptionCollection) Next() (*RkeK8sServiceOptionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RkeK8sServiceOptionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RkeK8sServiceOptionClient) ByID(id string) (*RkeK8sServiceOption, error) {
	resp := &RkeK8sServiceOption{}
	err := c.apiClient.Ops.DoByID(RkeK8sServiceOptionType, id, resp)
	return resp, err
}

func (c *RkeK8sServiceOptionClient) Delete(container *RkeK8sServiceOption) error {
	return c.apiClient.Ops.DoResourceDelete(RkeK8sServiceOptionType, &container.Resource)
}
