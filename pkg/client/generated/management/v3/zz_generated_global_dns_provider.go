package client

import (
	"github.com/rancher/norman/types"
)

const (
	GlobalDnsProviderType                          = "globalDnsProvider"
	GlobalDnsProviderFieldAlidnsProviderConfig     = "alidnsProviderConfig"
	GlobalDnsProviderFieldAnnotations              = "annotations"
	GlobalDnsProviderFieldCloudflareProviderConfig = "cloudflareProviderConfig"
	GlobalDnsProviderFieldCreated                  = "created"
	GlobalDnsProviderFieldCreatorID                = "creatorId"
	GlobalDnsProviderFieldLabels                   = "labels"
	GlobalDnsProviderFieldMembers                  = "members"
	GlobalDnsProviderFieldName                     = "name"
	GlobalDnsProviderFieldOwnerReferences          = "ownerReferences"
	GlobalDnsProviderFieldRemoved                  = "removed"
	GlobalDnsProviderFieldRootDomain               = "rootDomain"
	GlobalDnsProviderFieldRoute53ProviderConfig    = "route53ProviderConfig"
	GlobalDnsProviderFieldUUID                     = "uuid"
)

type GlobalDnsProvider struct {
	types.Resource
	AlidnsProviderConfig     *AlidnsProviderConfig     `json:"alidnsProviderConfig,omitempty" yaml:"alidnsProviderConfig,omitempty"`
	Annotations              map[string]string         `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	CloudflareProviderConfig *CloudflareProviderConfig `json:"cloudflareProviderConfig,omitempty" yaml:"cloudflareProviderConfig,omitempty"`
	Created                  string                    `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                string                    `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels                   map[string]string         `json:"labels,omitempty" yaml:"labels,omitempty"`
	Members                  []Member                  `json:"members,omitempty" yaml:"members,omitempty"`
	Name                     string                    `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences          []OwnerReference          `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed                  string                    `json:"removed,omitempty" yaml:"removed,omitempty"`
	RootDomain               string                    `json:"rootDomain,omitempty" yaml:"rootDomain,omitempty"`
	Route53ProviderConfig    *Route53ProviderConfig    `json:"route53ProviderConfig,omitempty" yaml:"route53ProviderConfig,omitempty"`
	UUID                     string                    `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type GlobalDnsProviderCollection struct {
	types.Collection
	Data   []GlobalDnsProvider `json:"data,omitempty"`
	client *GlobalDnsProviderClient
}

type GlobalDnsProviderClient struct {
	apiClient *Client
}

type GlobalDnsProviderOperations interface {
	List(opts *types.ListOpts) (*GlobalDnsProviderCollection, error)
	ListAll(opts *types.ListOpts) (*GlobalDnsProviderCollection, error)
	Create(opts *GlobalDnsProvider) (*GlobalDnsProvider, error)
	Update(existing *GlobalDnsProvider, updates interface{}) (*GlobalDnsProvider, error)
	Replace(existing *GlobalDnsProvider) (*GlobalDnsProvider, error)
	ByID(id string) (*GlobalDnsProvider, error)
	Delete(container *GlobalDnsProvider) error
}

func newGlobalDnsProviderClient(apiClient *Client) *GlobalDnsProviderClient {
	return &GlobalDnsProviderClient{
		apiClient: apiClient,
	}
}

func (c *GlobalDnsProviderClient) Create(container *GlobalDnsProvider) (*GlobalDnsProvider, error) {
	resp := &GlobalDnsProvider{}
	err := c.apiClient.Ops.DoCreate(GlobalDnsProviderType, container, resp)
	return resp, err
}

func (c *GlobalDnsProviderClient) Update(existing *GlobalDnsProvider, updates interface{}) (*GlobalDnsProvider, error) {
	resp := &GlobalDnsProvider{}
	err := c.apiClient.Ops.DoUpdate(GlobalDnsProviderType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GlobalDnsProviderClient) Replace(obj *GlobalDnsProvider) (*GlobalDnsProvider, error) {
	resp := &GlobalDnsProvider{}
	err := c.apiClient.Ops.DoReplace(GlobalDnsProviderType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *GlobalDnsProviderClient) List(opts *types.ListOpts) (*GlobalDnsProviderCollection, error) {
	resp := &GlobalDnsProviderCollection{}
	err := c.apiClient.Ops.DoList(GlobalDnsProviderType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *GlobalDnsProviderClient) ListAll(opts *types.ListOpts) (*GlobalDnsProviderCollection, error) {
	resp := &GlobalDnsProviderCollection{}
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

func (cc *GlobalDnsProviderCollection) Next() (*GlobalDnsProviderCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GlobalDnsProviderCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GlobalDnsProviderClient) ByID(id string) (*GlobalDnsProvider, error) {
	resp := &GlobalDnsProvider{}
	err := c.apiClient.Ops.DoByID(GlobalDnsProviderType, id, resp)
	return resp, err
}

func (c *GlobalDnsProviderClient) Delete(container *GlobalDnsProvider) error {
	return c.apiClient.Ops.DoResourceDelete(GlobalDnsProviderType, &container.Resource)
}
