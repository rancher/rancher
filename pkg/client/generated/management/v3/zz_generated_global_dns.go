package client

import (
	"github.com/rancher/norman/types"
)

const (
	GlobalDnsType                      = "globalDns"
	GlobalDnsFieldAnnotations          = "annotations"
	GlobalDnsFieldCreated              = "created"
	GlobalDnsFieldCreatorID            = "creatorId"
	GlobalDnsFieldFQDN                 = "fqdn"
	GlobalDnsFieldLabels               = "labels"
	GlobalDnsFieldMembers              = "members"
	GlobalDnsFieldMultiClusterAppID    = "multiClusterAppId"
	GlobalDnsFieldName                 = "name"
	GlobalDnsFieldOwnerReferences      = "ownerReferences"
	GlobalDnsFieldProjectIDs           = "projectIds"
	GlobalDnsFieldProviderID           = "providerId"
	GlobalDnsFieldRemoved              = "removed"
	GlobalDnsFieldState                = "state"
	GlobalDnsFieldStatus               = "status"
	GlobalDnsFieldTTL                  = "ttl"
	GlobalDnsFieldTransitioning        = "transitioning"
	GlobalDnsFieldTransitioningMessage = "transitioningMessage"
	GlobalDnsFieldUUID                 = "uuid"
)

type GlobalDns struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	FQDN                 string            `json:"fqdn,omitempty" yaml:"fqdn,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Members              []Member          `json:"members,omitempty" yaml:"members,omitempty"`
	MultiClusterAppID    string            `json:"multiClusterAppId,omitempty" yaml:"multiClusterAppId,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectIDs           []string          `json:"projectIds,omitempty" yaml:"projectIds,omitempty"`
	ProviderID           string            `json:"providerId,omitempty" yaml:"providerId,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *GlobalDNSStatus  `json:"status,omitempty" yaml:"status,omitempty"`
	TTL                  int64             `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type GlobalDnsCollection struct {
	types.Collection
	Data   []GlobalDns `json:"data,omitempty"`
	client *GlobalDnsClient
}

type GlobalDnsClient struct {
	apiClient *Client
}

type GlobalDnsOperations interface {
	List(opts *types.ListOpts) (*GlobalDnsCollection, error)
	ListAll(opts *types.ListOpts) (*GlobalDnsCollection, error)
	Create(opts *GlobalDns) (*GlobalDns, error)
	Update(existing *GlobalDns, updates interface{}) (*GlobalDns, error)
	Replace(existing *GlobalDns) (*GlobalDns, error)
	ByID(id string) (*GlobalDns, error)
	Delete(container *GlobalDns) error

	ActionAddProjects(resource *GlobalDns, input *UpdateGlobalDNSTargetsInput) error

	ActionRemoveProjects(resource *GlobalDns, input *UpdateGlobalDNSTargetsInput) error
}

func newGlobalDnsClient(apiClient *Client) *GlobalDnsClient {
	return &GlobalDnsClient{
		apiClient: apiClient,
	}
}

func (c *GlobalDnsClient) Create(container *GlobalDns) (*GlobalDns, error) {
	resp := &GlobalDns{}
	err := c.apiClient.Ops.DoCreate(GlobalDnsType, container, resp)
	return resp, err
}

func (c *GlobalDnsClient) Update(existing *GlobalDns, updates interface{}) (*GlobalDns, error) {
	resp := &GlobalDns{}
	err := c.apiClient.Ops.DoUpdate(GlobalDnsType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GlobalDnsClient) Replace(obj *GlobalDns) (*GlobalDns, error) {
	resp := &GlobalDns{}
	err := c.apiClient.Ops.DoReplace(GlobalDnsType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *GlobalDnsClient) List(opts *types.ListOpts) (*GlobalDnsCollection, error) {
	resp := &GlobalDnsCollection{}
	err := c.apiClient.Ops.DoList(GlobalDnsType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *GlobalDnsClient) ListAll(opts *types.ListOpts) (*GlobalDnsCollection, error) {
	resp := &GlobalDnsCollection{}
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

func (cc *GlobalDnsCollection) Next() (*GlobalDnsCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GlobalDnsCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GlobalDnsClient) ByID(id string) (*GlobalDns, error) {
	resp := &GlobalDns{}
	err := c.apiClient.Ops.DoByID(GlobalDnsType, id, resp)
	return resp, err
}

func (c *GlobalDnsClient) Delete(container *GlobalDns) error {
	return c.apiClient.Ops.DoResourceDelete(GlobalDnsType, &container.Resource)
}

func (c *GlobalDnsClient) ActionAddProjects(resource *GlobalDns, input *UpdateGlobalDNSTargetsInput) error {
	err := c.apiClient.Ops.DoAction(GlobalDnsType, "addProjects", &resource.Resource, input, nil)
	return err
}

func (c *GlobalDnsClient) ActionRemoveProjects(resource *GlobalDns, input *UpdateGlobalDNSTargetsInput) error {
	err := c.apiClient.Ops.DoAction(GlobalDnsType, "removeProjects", &resource.Resource, input, nil)
	return err
}
