package client

import (
	"github.com/rancher/norman/types"
)

const (
	GlobalDNSType                      = "globalDns"
	GlobalDNSFieldAnnotations          = "annotations"
	GlobalDNSFieldCreated              = "created"
	GlobalDNSFieldCreatorID            = "creatorId"
	GlobalDNSFieldFQDN                 = "fqdn"
	GlobalDNSFieldLabels               = "labels"
	GlobalDNSFieldMembers              = "members"
	GlobalDNSFieldMultiClusterAppID    = "multiClusterAppId"
	GlobalDNSFieldName                 = "name"
	GlobalDNSFieldOwnerReferences      = "ownerReferences"
	GlobalDNSFieldProjectIDs           = "projectIds"
	GlobalDNSFieldProviderID           = "providerId"
	GlobalDNSFieldRemoved              = "removed"
	GlobalDNSFieldState                = "state"
	GlobalDNSFieldStatus               = "status"
	GlobalDNSFieldTTL                  = "ttl"
	GlobalDNSFieldTransitioning        = "transitioning"
	GlobalDNSFieldTransitioningMessage = "transitioningMessage"
	GlobalDNSFieldUUID                 = "uuid"
)

type GlobalDNS struct {
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

type GlobalDNSCollection struct {
	types.Collection
	Data   []GlobalDNS `json:"data,omitempty"`
	client *GlobalDNSClient
}

type GlobalDNSClient struct {
	apiClient *Client
}

type GlobalDNSOperations interface {
	List(opts *types.ListOpts) (*GlobalDNSCollection, error)
	ListAll(opts *types.ListOpts) (*GlobalDNSCollection, error)
	Create(opts *GlobalDNS) (*GlobalDNS, error)
	Update(existing *GlobalDNS, updates interface{}) (*GlobalDNS, error)
	Replace(existing *GlobalDNS) (*GlobalDNS, error)
	ByID(id string) (*GlobalDNS, error)
	Delete(container *GlobalDNS) error

	ActionAddProjects(resource *GlobalDNS, input *UpdateGlobalDNSTargetsInput) error

	ActionRemoveProjects(resource *GlobalDNS, input *UpdateGlobalDNSTargetsInput) error
}

func newGlobalDNSClient(apiClient *Client) *GlobalDNSClient {
	return &GlobalDNSClient{
		apiClient: apiClient,
	}
}

func (c *GlobalDNSClient) Create(container *GlobalDNS) (*GlobalDNS, error) {
	resp := &GlobalDNS{}
	err := c.apiClient.Ops.DoCreate(GlobalDNSType, container, resp)
	return resp, err
}

func (c *GlobalDNSClient) Update(existing *GlobalDNS, updates interface{}) (*GlobalDNS, error) {
	resp := &GlobalDNS{}
	err := c.apiClient.Ops.DoUpdate(GlobalDNSType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GlobalDNSClient) Replace(obj *GlobalDNS) (*GlobalDNS, error) {
	resp := &GlobalDNS{}
	err := c.apiClient.Ops.DoReplace(GlobalDNSType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *GlobalDNSClient) List(opts *types.ListOpts) (*GlobalDNSCollection, error) {
	resp := &GlobalDNSCollection{}
	err := c.apiClient.Ops.DoList(GlobalDNSType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *GlobalDNSClient) ListAll(opts *types.ListOpts) (*GlobalDNSCollection, error) {
	resp := &GlobalDNSCollection{}
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

func (cc *GlobalDNSCollection) Next() (*GlobalDNSCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GlobalDNSCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GlobalDNSClient) ByID(id string) (*GlobalDNS, error) {
	resp := &GlobalDNS{}
	err := c.apiClient.Ops.DoByID(GlobalDNSType, id, resp)
	return resp, err
}

func (c *GlobalDNSClient) Delete(container *GlobalDNS) error {
	return c.apiClient.Ops.DoResourceDelete(GlobalDNSType, &container.Resource)
}

func (c *GlobalDNSClient) ActionAddProjects(resource *GlobalDNS, input *UpdateGlobalDNSTargetsInput) error {
	err := c.apiClient.Ops.DoAction(GlobalDNSType, "addProjects", &resource.Resource, input, nil)
	return err
}

func (c *GlobalDNSClient) ActionRemoveProjects(resource *GlobalDNS, input *UpdateGlobalDNSTargetsInput) error {
	err := c.apiClient.Ops.DoAction(GlobalDNSType, "removeProjects", &resource.Resource, input, nil)
	return err
}
