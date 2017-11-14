package client

import (
	"github.com/rancher/norman/types"
)

const (
	GithubCredentialType                 = "githubCredential"
	GithubCredentialFieldAnnotations     = "annotations"
	GithubCredentialFieldCode            = "code"
	GithubCredentialFieldCreated         = "created"
	GithubCredentialFieldFinalizers      = "finalizers"
	GithubCredentialFieldLabels          = "labels"
	GithubCredentialFieldName            = "name"
	GithubCredentialFieldOwnerReferences = "ownerReferences"
	GithubCredentialFieldRemoved         = "removed"
	GithubCredentialFieldResourcePath    = "resourcePath"
	GithubCredentialFieldUuid            = "uuid"
)

type GithubCredential struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Code            string            `json:"code,omitempty"`
	Created         string            `json:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	ResourcePath    string            `json:"resourcePath,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type GithubCredentialCollection struct {
	types.Collection
	Data   []GithubCredential `json:"data,omitempty"`
	client *GithubCredentialClient
}

type GithubCredentialClient struct {
	apiClient *Client
}

type GithubCredentialOperations interface {
	List(opts *types.ListOpts) (*GithubCredentialCollection, error)
	Create(opts *GithubCredential) (*GithubCredential, error)
	Update(existing *GithubCredential, updates interface{}) (*GithubCredential, error)
	ByID(id string) (*GithubCredential, error)
	Delete(container *GithubCredential) error
}

func newGithubCredentialClient(apiClient *Client) *GithubCredentialClient {
	return &GithubCredentialClient{
		apiClient: apiClient,
	}
}

func (c *GithubCredentialClient) Create(container *GithubCredential) (*GithubCredential, error) {
	resp := &GithubCredential{}
	err := c.apiClient.Ops.DoCreate(GithubCredentialType, container, resp)
	return resp, err
}

func (c *GithubCredentialClient) Update(existing *GithubCredential, updates interface{}) (*GithubCredential, error) {
	resp := &GithubCredential{}
	err := c.apiClient.Ops.DoUpdate(GithubCredentialType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GithubCredentialClient) List(opts *types.ListOpts) (*GithubCredentialCollection, error) {
	resp := &GithubCredentialCollection{}
	err := c.apiClient.Ops.DoList(GithubCredentialType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *GithubCredentialCollection) Next() (*GithubCredentialCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GithubCredentialCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GithubCredentialClient) ByID(id string) (*GithubCredential, error) {
	resp := &GithubCredential{}
	err := c.apiClient.Ops.DoByID(GithubCredentialType, id, resp)
	return resp, err
}

func (c *GithubCredentialClient) Delete(container *GithubCredential) error {
	return c.apiClient.Ops.DoResourceDelete(GithubCredentialType, &container.Resource)
}
