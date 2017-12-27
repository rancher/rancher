package client

import (
	"github.com/rancher/norman/types"
)

const (
	ServiceAccountTokenType                 = "serviceAccountToken"
	ServiceAccountTokenField                = "creatorId"
	ServiceAccountTokenFieldAccountName     = "accountName"
	ServiceAccountTokenFieldAccountUID      = "accountUid"
	ServiceAccountTokenFieldAnnotations     = "annotations"
	ServiceAccountTokenFieldCACRT           = "caCrt"
	ServiceAccountTokenFieldCreated         = "created"
	ServiceAccountTokenFieldFinalizers      = "finalizers"
	ServiceAccountTokenFieldLabels          = "labels"
	ServiceAccountTokenFieldName            = "name"
	ServiceAccountTokenFieldNamespaceId     = "namespaceId"
	ServiceAccountTokenFieldOwnerReferences = "ownerReferences"
	ServiceAccountTokenFieldProjectID       = "projectId"
	ServiceAccountTokenFieldRemoved         = "removed"
	ServiceAccountTokenFieldToken           = "token"
	ServiceAccountTokenFieldUuid            = "uuid"
)

type ServiceAccountToken struct {
	types.Resource
	string          `json:"creatorId,omitempty"`
	AccountName     string            `json:"accountName,omitempty"`
	AccountUID      string            `json:"accountUid,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	CACRT           string            `json:"caCrt,omitempty"`
	Created         string            `json:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectID       string            `json:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Token           string            `json:"token,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type ServiceAccountTokenCollection struct {
	types.Collection
	Data   []ServiceAccountToken `json:"data,omitempty"`
	client *ServiceAccountTokenClient
}

type ServiceAccountTokenClient struct {
	apiClient *Client
}

type ServiceAccountTokenOperations interface {
	List(opts *types.ListOpts) (*ServiceAccountTokenCollection, error)
	Create(opts *ServiceAccountToken) (*ServiceAccountToken, error)
	Update(existing *ServiceAccountToken, updates interface{}) (*ServiceAccountToken, error)
	ByID(id string) (*ServiceAccountToken, error)
	Delete(container *ServiceAccountToken) error
}

func newServiceAccountTokenClient(apiClient *Client) *ServiceAccountTokenClient {
	return &ServiceAccountTokenClient{
		apiClient: apiClient,
	}
}

func (c *ServiceAccountTokenClient) Create(container *ServiceAccountToken) (*ServiceAccountToken, error) {
	resp := &ServiceAccountToken{}
	err := c.apiClient.Ops.DoCreate(ServiceAccountTokenType, container, resp)
	return resp, err
}

func (c *ServiceAccountTokenClient) Update(existing *ServiceAccountToken, updates interface{}) (*ServiceAccountToken, error) {
	resp := &ServiceAccountToken{}
	err := c.apiClient.Ops.DoUpdate(ServiceAccountTokenType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ServiceAccountTokenClient) List(opts *types.ListOpts) (*ServiceAccountTokenCollection, error) {
	resp := &ServiceAccountTokenCollection{}
	err := c.apiClient.Ops.DoList(ServiceAccountTokenType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ServiceAccountTokenCollection) Next() (*ServiceAccountTokenCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ServiceAccountTokenCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ServiceAccountTokenClient) ByID(id string) (*ServiceAccountToken, error) {
	resp := &ServiceAccountToken{}
	err := c.apiClient.Ops.DoByID(ServiceAccountTokenType, id, resp)
	return resp, err
}

func (c *ServiceAccountTokenClient) Delete(container *ServiceAccountToken) error {
	return c.apiClient.Ops.DoResourceDelete(ServiceAccountTokenType, &container.Resource)
}
