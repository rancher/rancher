package client

import (
	"github.com/rancher/norman/types"
)

const (
	DockerCredentialType                 = "dockerCredential"
	DockerCredentialFieldAnnotations     = "annotations"
	DockerCredentialFieldCreated         = "created"
	DockerCredentialFieldCreatorID       = "creatorId"
	DockerCredentialFieldDescription     = "description"
	DockerCredentialFieldLabels          = "labels"
	DockerCredentialFieldName            = "name"
	DockerCredentialFieldNamespaceId     = "namespaceId"
	DockerCredentialFieldOwnerReferences = "ownerReferences"
	DockerCredentialFieldProjectID       = "projectId"
	DockerCredentialFieldRegistries      = "registries"
	DockerCredentialFieldRemoved         = "removed"
	DockerCredentialFieldUUID            = "uuid"
)

type DockerCredential struct {
	types.Resource
	Annotations     map[string]string             `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string                        `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string                        `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description     string                        `json:"description,omitempty" yaml:"description,omitempty"`
	Labels          map[string]string             `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string                        `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string                        `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference              `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID       string                        `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Registries      map[string]RegistryCredential `json:"registries,omitempty" yaml:"registries,omitempty"`
	Removed         string                        `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string                        `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type DockerCredentialCollection struct {
	types.Collection
	Data   []DockerCredential `json:"data,omitempty"`
	client *DockerCredentialClient
}

type DockerCredentialClient struct {
	apiClient *Client
}

type DockerCredentialOperations interface {
	List(opts *types.ListOpts) (*DockerCredentialCollection, error)
	ListAll(opts *types.ListOpts) (*DockerCredentialCollection, error)
	Create(opts *DockerCredential) (*DockerCredential, error)
	Update(existing *DockerCredential, updates interface{}) (*DockerCredential, error)
	Replace(existing *DockerCredential) (*DockerCredential, error)
	ByID(id string) (*DockerCredential, error)
	Delete(container *DockerCredential) error
}

func newDockerCredentialClient(apiClient *Client) *DockerCredentialClient {
	return &DockerCredentialClient{
		apiClient: apiClient,
	}
}

func (c *DockerCredentialClient) Create(container *DockerCredential) (*DockerCredential, error) {
	resp := &DockerCredential{}
	err := c.apiClient.Ops.DoCreate(DockerCredentialType, container, resp)
	return resp, err
}

func (c *DockerCredentialClient) Update(existing *DockerCredential, updates interface{}) (*DockerCredential, error) {
	resp := &DockerCredential{}
	err := c.apiClient.Ops.DoUpdate(DockerCredentialType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DockerCredentialClient) Replace(obj *DockerCredential) (*DockerCredential, error) {
	resp := &DockerCredential{}
	err := c.apiClient.Ops.DoReplace(DockerCredentialType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *DockerCredentialClient) List(opts *types.ListOpts) (*DockerCredentialCollection, error) {
	resp := &DockerCredentialCollection{}
	err := c.apiClient.Ops.DoList(DockerCredentialType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *DockerCredentialClient) ListAll(opts *types.ListOpts) (*DockerCredentialCollection, error) {
	resp := &DockerCredentialCollection{}
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

func (cc *DockerCredentialCollection) Next() (*DockerCredentialCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &DockerCredentialCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *DockerCredentialClient) ByID(id string) (*DockerCredential, error) {
	resp := &DockerCredential{}
	err := c.apiClient.Ops.DoByID(DockerCredentialType, id, resp)
	return resp, err
}

func (c *DockerCredentialClient) Delete(container *DockerCredential) error {
	return c.apiClient.Ops.DoResourceDelete(DockerCredentialType, &container.Resource)
}
