package client

import (
	"github.com/rancher/norman/types"
)

const (
	GlobalDNSProviderType                          = "globalDnsProvider"
	GlobalDNSProviderFieldAlidnsProviderConfig     = "alidnsProviderConfig"
	GlobalDNSProviderFieldAnnotations              = "annotations"
	GlobalDNSProviderFieldCloudflareProviderConfig = "cloudflareProviderConfig"
	GlobalDNSProviderFieldCreated                  = "created"
	GlobalDNSProviderFieldCreatorID                = "creatorId"
	GlobalDNSProviderFieldLabels                   = "labels"
	GlobalDNSProviderFieldMembers                  = "members"
	GlobalDNSProviderFieldName                     = "name"
	GlobalDNSProviderFieldOwnerReferences          = "ownerReferences"
	GlobalDNSProviderFieldRemoved                  = "removed"
	GlobalDNSProviderFieldRootDomain               = "rootDomain"
	GlobalDNSProviderFieldRoute53ProviderConfig    = "route53ProviderConfig"
	GlobalDNSProviderFieldUUID                     = "uuid"
)

type GlobalDNSProvider struct {
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

type GlobalDNSProviderCollection struct {
	types.Collection
	Data   []GlobalDNSProvider `json:"data,omitempty"`
	client *GlobalDNSProviderClient
}

type GlobalDNSProviderClient struct {
	apiClient *Client
}

type GlobalDNSProviderOperations interface {
	List(opts *types.ListOpts) (*GlobalDNSProviderCollection, error)
	ListAll(opts *types.ListOpts) (*GlobalDNSProviderCollection, error)
	Create(opts *GlobalDNSProvider) (*GlobalDNSProvider, error)
	Update(existing *GlobalDNSProvider, updates interface{}) (*GlobalDNSProvider, error)
	Replace(existing *GlobalDNSProvider) (*GlobalDNSProvider, error)
	ByID(id string) (*GlobalDNSProvider, error)
	Delete(container *GlobalDNSProvider) error
}

func newGlobalDNSProviderClient(apiClient *Client) *GlobalDNSProviderClient {
	return &GlobalDNSProviderClient{
		apiClient: apiClient,
	}
}

func (c *GlobalDNSProviderClient) Create(container *GlobalDNSProvider) (*GlobalDNSProvider, error) {
	resp := &GlobalDNSProvider{}
	err := c.apiClient.Ops.DoCreate(GlobalDNSProviderType, container, resp)
	return resp, err
}

func (c *GlobalDNSProviderClient) Update(existing *GlobalDNSProvider, updates interface{}) (*GlobalDNSProvider, error) {
	resp := &GlobalDNSProvider{}
	err := c.apiClient.Ops.DoUpdate(GlobalDNSProviderType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GlobalDNSProviderClient) Replace(obj *GlobalDNSProvider) (*GlobalDNSProvider, error) {
	resp := &GlobalDNSProvider{}
	err := c.apiClient.Ops.DoReplace(GlobalDNSProviderType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *GlobalDNSProviderClient) List(opts *types.ListOpts) (*GlobalDNSProviderCollection, error) {
	resp := &GlobalDNSProviderCollection{}
	err := c.apiClient.Ops.DoList(GlobalDNSProviderType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *GlobalDNSProviderClient) ListAll(opts *types.ListOpts) (*GlobalDNSProviderCollection, error) {
	resp := &GlobalDNSProviderCollection{}
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

func (cc *GlobalDNSProviderCollection) Next() (*GlobalDNSProviderCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GlobalDNSProviderCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GlobalDNSProviderClient) ByID(id string) (*GlobalDNSProvider, error) {
	resp := &GlobalDNSProvider{}
	err := c.apiClient.Ops.DoByID(GlobalDNSProviderType, id, resp)
	return resp, err
}

func (c *GlobalDNSProviderClient) Delete(container *GlobalDNSProvider) error {
	return c.apiClient.Ops.DoResourceDelete(GlobalDNSProviderType, &container.Resource)
}
