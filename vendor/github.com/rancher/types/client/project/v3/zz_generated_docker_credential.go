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
	DockerCredentialFieldFinalizers      = "finalizers"
	DockerCredentialFieldLabels          = "labels"
	DockerCredentialFieldName            = "name"
	DockerCredentialFieldNamespaceId     = "namespaceId"
	DockerCredentialFieldOwnerReferences = "ownerReferences"
	DockerCredentialFieldProjectID       = "projectId"
	DockerCredentialFieldRegistries      = "registries"
	DockerCredentialFieldRemoved         = "removed"
	DockerCredentialFieldUuid            = "uuid"
)

type DockerCredential struct {
	types.Resource
	Annotations     map[string]string             `json:"annotations,omitempty"`
	Created         string                        `json:"created,omitempty"`
	CreatorID       string                        `json:"creatorId,omitempty"`
	Description     string                        `json:"description,omitempty"`
	Finalizers      []string                      `json:"finalizers,omitempty"`
	Labels          map[string]string             `json:"labels,omitempty"`
	Name            string                        `json:"name,omitempty"`
	NamespaceId     string                        `json:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference              `json:"ownerReferences,omitempty"`
	ProjectID       string                        `json:"projectId,omitempty"`
	Registries      map[string]RegistryCredential `json:"registries,omitempty"`
	Removed         string                        `json:"removed,omitempty"`
	Uuid            string                        `json:"uuid,omitempty"`
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
	Create(opts *DockerCredential) (*DockerCredential, error)
	Update(existing *DockerCredential, updates interface{}) (*DockerCredential, error)
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

func (c *DockerCredentialClient) List(opts *types.ListOpts) (*DockerCredentialCollection, error) {
	resp := &DockerCredentialCollection{}
	err := c.apiClient.Ops.DoList(DockerCredentialType, opts, resp)
	resp.client = c
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
