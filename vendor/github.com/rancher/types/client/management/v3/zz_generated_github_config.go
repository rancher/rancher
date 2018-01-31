package client

import (
	"github.com/rancher/norman/types"
)

const (
	GithubConfigType                 = "githubConfig"
	GithubConfigFieldAnnotations     = "annotations"
	GithubConfigFieldClientID        = "clientId"
	GithubConfigFieldClientSecret    = "clientSecret"
	GithubConfigFieldCreated         = "created"
	GithubConfigFieldCreatorID       = "creatorId"
	GithubConfigFieldEnabled         = "enabled"
	GithubConfigFieldHostname        = "hostname"
	GithubConfigFieldLabels          = "labels"
	GithubConfigFieldName            = "name"
	GithubConfigFieldOwnerReferences = "ownerReferences"
	GithubConfigFieldRemoved         = "removed"
	GithubConfigFieldScheme          = "scheme"
	GithubConfigFieldType            = "type"
	GithubConfigFieldUuid            = "uuid"
)

type GithubConfig struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	ClientID        string            `json:"clientId,omitempty"`
	ClientSecret    string            `json:"clientSecret,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Enabled         *bool             `json:"enabled,omitempty"`
	Hostname        string            `json:"hostname,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Scheme          string            `json:"scheme,omitempty"`
	Type            string            `json:"type,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type GithubConfigCollection struct {
	types.Collection
	Data   []GithubConfig `json:"data,omitempty"`
	client *GithubConfigClient
}

type GithubConfigClient struct {
	apiClient *Client
}

type GithubConfigOperations interface {
	List(opts *types.ListOpts) (*GithubConfigCollection, error)
	Create(opts *GithubConfig) (*GithubConfig, error)
	Update(existing *GithubConfig, updates interface{}) (*GithubConfig, error)
	ByID(id string) (*GithubConfig, error)
	Delete(container *GithubConfig) error

	ActionConfigureTest(*GithubConfig, *GithubConfigTestInput) (*GithubConfig, error)

	ActionTestAndApply(*GithubConfig, *GithubConfigApplyInput) (*GithubConfig, error)
}

func newGithubConfigClient(apiClient *Client) *GithubConfigClient {
	return &GithubConfigClient{
		apiClient: apiClient,
	}
}

func (c *GithubConfigClient) Create(container *GithubConfig) (*GithubConfig, error) {
	resp := &GithubConfig{}
	err := c.apiClient.Ops.DoCreate(GithubConfigType, container, resp)
	return resp, err
}

func (c *GithubConfigClient) Update(existing *GithubConfig, updates interface{}) (*GithubConfig, error) {
	resp := &GithubConfig{}
	err := c.apiClient.Ops.DoUpdate(GithubConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GithubConfigClient) List(opts *types.ListOpts) (*GithubConfigCollection, error) {
	resp := &GithubConfigCollection{}
	err := c.apiClient.Ops.DoList(GithubConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *GithubConfigCollection) Next() (*GithubConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GithubConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GithubConfigClient) ByID(id string) (*GithubConfig, error) {
	resp := &GithubConfig{}
	err := c.apiClient.Ops.DoByID(GithubConfigType, id, resp)
	return resp, err
}

func (c *GithubConfigClient) Delete(container *GithubConfig) error {
	return c.apiClient.Ops.DoResourceDelete(GithubConfigType, &container.Resource)
}

func (c *GithubConfigClient) ActionConfigureTest(resource *GithubConfig, input *GithubConfigTestInput) (*GithubConfig, error) {

	resp := &GithubConfig{}

	err := c.apiClient.Ops.DoAction(GithubConfigType, "configureTest", &resource.Resource, input, resp)

	return resp, err
}

func (c *GithubConfigClient) ActionTestAndApply(resource *GithubConfig, input *GithubConfigApplyInput) (*GithubConfig, error) {

	resp := &GithubConfig{}

	err := c.apiClient.Ops.DoAction(GithubConfigType, "testAndApply", &resource.Resource, input, resp)

	return resp, err
}
