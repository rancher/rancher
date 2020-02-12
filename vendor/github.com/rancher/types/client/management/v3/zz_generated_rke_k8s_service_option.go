package client

import (
	"github.com/rancher/norman/types"
)

const (
	RKEK8sServiceOptionType                 = "rkeK8sServiceOption"
	RKEK8sServiceOptionFieldAnnotations     = "annotations"
	RKEK8sServiceOptionFieldCreated         = "created"
	RKEK8sServiceOptionFieldCreatorID       = "creatorId"
	RKEK8sServiceOptionFieldLabels          = "labels"
	RKEK8sServiceOptionFieldName            = "name"
	RKEK8sServiceOptionFieldOwnerReferences = "ownerReferences"
	RKEK8sServiceOptionFieldRemoved         = "removed"
	RKEK8sServiceOptionFieldServiceOptions  = "serviceOptions"
	RKEK8sServiceOptionFieldUUID            = "uuid"
)

type RKEK8sServiceOption struct {
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

type RKEK8sServiceOptionCollection struct {
	types.Collection
	Data   []RKEK8sServiceOption `json:"data,omitempty"`
	client *RKEK8sServiceOptionClient
}

type RKEK8sServiceOptionClient struct {
	apiClient *Client
}

type RKEK8sServiceOptionOperations interface {
	List(opts *types.ListOpts) (*RKEK8sServiceOptionCollection, error)
	ListAll(opts *types.ListOpts) (*RKEK8sServiceOptionCollection, error)
	Create(opts *RKEK8sServiceOption) (*RKEK8sServiceOption, error)
	Update(existing *RKEK8sServiceOption, updates interface{}) (*RKEK8sServiceOption, error)
	Replace(existing *RKEK8sServiceOption) (*RKEK8sServiceOption, error)
	ByID(id string) (*RKEK8sServiceOption, error)
	Delete(container *RKEK8sServiceOption) error
}

func newRKEK8sServiceOptionClient(apiClient *Client) *RKEK8sServiceOptionClient {
	return &RKEK8sServiceOptionClient{
		apiClient: apiClient,
	}
}

func (c *RKEK8sServiceOptionClient) Create(container *RKEK8sServiceOption) (*RKEK8sServiceOption, error) {
	resp := &RKEK8sServiceOption{}
	err := c.apiClient.Ops.DoCreate(RKEK8sServiceOptionType, container, resp)
	return resp, err
}

func (c *RKEK8sServiceOptionClient) Update(existing *RKEK8sServiceOption, updates interface{}) (*RKEK8sServiceOption, error) {
	resp := &RKEK8sServiceOption{}
	err := c.apiClient.Ops.DoUpdate(RKEK8sServiceOptionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RKEK8sServiceOptionClient) Replace(obj *RKEK8sServiceOption) (*RKEK8sServiceOption, error) {
	resp := &RKEK8sServiceOption{}
	err := c.apiClient.Ops.DoReplace(RKEK8sServiceOptionType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RKEK8sServiceOptionClient) List(opts *types.ListOpts) (*RKEK8sServiceOptionCollection, error) {
	resp := &RKEK8sServiceOptionCollection{}
	err := c.apiClient.Ops.DoList(RKEK8sServiceOptionType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *RKEK8sServiceOptionClient) ListAll(opts *types.ListOpts) (*RKEK8sServiceOptionCollection, error) {
	resp := &RKEK8sServiceOptionCollection{}
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

func (cc *RKEK8sServiceOptionCollection) Next() (*RKEK8sServiceOptionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RKEK8sServiceOptionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RKEK8sServiceOptionClient) ByID(id string) (*RKEK8sServiceOption, error) {
	resp := &RKEK8sServiceOption{}
	err := c.apiClient.Ops.DoByID(RKEK8sServiceOptionType, id, resp)
	return resp, err
}

func (c *RKEK8sServiceOptionClient) Delete(container *RKEK8sServiceOption) error {
	return c.apiClient.Ops.DoResourceDelete(RKEK8sServiceOptionType, &container.Resource)
}
