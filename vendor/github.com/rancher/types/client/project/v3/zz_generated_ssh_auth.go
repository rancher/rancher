package client

import (
	"github.com/rancher/norman/types"
)

const (
	SSHAuthType                 = "sshAuth"
	SSHAuthFieldAnnotations     = "annotations"
	SSHAuthFieldCreated         = "created"
	SSHAuthFieldFinalizers      = "finalizers"
	SSHAuthFieldLabels          = "labels"
	SSHAuthFieldName            = "name"
	SSHAuthFieldNamespace       = "namespace"
	SSHAuthFieldOwnerReferences = "ownerReferences"
	SSHAuthFieldPrivateKey      = "privateKey"
	SSHAuthFieldProjectID       = "projectId"
	SSHAuthFieldRemoved         = "removed"
	SSHAuthFieldUuid            = "uuid"
)

type SSHAuth struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	PrivateKey      string            `json:"privateKey,omitempty"`
	ProjectID       string            `json:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type SSHAuthCollection struct {
	types.Collection
	Data   []SSHAuth `json:"data,omitempty"`
	client *SSHAuthClient
}

type SSHAuthClient struct {
	apiClient *Client
}

type SSHAuthOperations interface {
	List(opts *types.ListOpts) (*SSHAuthCollection, error)
	Create(opts *SSHAuth) (*SSHAuth, error)
	Update(existing *SSHAuth, updates interface{}) (*SSHAuth, error)
	ByID(id string) (*SSHAuth, error)
	Delete(container *SSHAuth) error
}

func newSSHAuthClient(apiClient *Client) *SSHAuthClient {
	return &SSHAuthClient{
		apiClient: apiClient,
	}
}

func (c *SSHAuthClient) Create(container *SSHAuth) (*SSHAuth, error) {
	resp := &SSHAuth{}
	err := c.apiClient.Ops.DoCreate(SSHAuthType, container, resp)
	return resp, err
}

func (c *SSHAuthClient) Update(existing *SSHAuth, updates interface{}) (*SSHAuth, error) {
	resp := &SSHAuth{}
	err := c.apiClient.Ops.DoUpdate(SSHAuthType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *SSHAuthClient) List(opts *types.ListOpts) (*SSHAuthCollection, error) {
	resp := &SSHAuthCollection{}
	err := c.apiClient.Ops.DoList(SSHAuthType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *SSHAuthCollection) Next() (*SSHAuthCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &SSHAuthCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *SSHAuthClient) ByID(id string) (*SSHAuth, error) {
	resp := &SSHAuth{}
	err := c.apiClient.Ops.DoByID(SSHAuthType, id, resp)
	return resp, err
}

func (c *SSHAuthClient) Delete(container *SSHAuth) error {
	return c.apiClient.Ops.DoResourceDelete(SSHAuthType, &container.Resource)
}
