package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterRoleTemplateBindingType                  = "clusterRoleTemplateBinding"
	ClusterRoleTemplateBindingFieldAnnotations      = "annotations"
	ClusterRoleTemplateBindingFieldClusterId        = "clusterId"
	ClusterRoleTemplateBindingFieldCreated          = "created"
	ClusterRoleTemplateBindingFieldCreatorID        = "creatorId"
	ClusterRoleTemplateBindingFieldLabels           = "labels"
	ClusterRoleTemplateBindingFieldName             = "name"
	ClusterRoleTemplateBindingFieldNamespaceId      = "namespaceId"
	ClusterRoleTemplateBindingFieldOwnerReferences  = "ownerReferences"
	ClusterRoleTemplateBindingFieldRemoved          = "removed"
	ClusterRoleTemplateBindingFieldRoleTemplateId   = "roleTemplateId"
	ClusterRoleTemplateBindingFieldSubjectKind      = "subjectKind"
	ClusterRoleTemplateBindingFieldSubjectName      = "subjectName"
	ClusterRoleTemplateBindingFieldSubjectNamespace = "subjectNamespace"
	ClusterRoleTemplateBindingFieldUuid             = "uuid"
)

type ClusterRoleTemplateBinding struct {
	types.Resource
	Annotations      map[string]string `json:"annotations,omitempty"`
	ClusterId        string            `json:"clusterId,omitempty"`
	Created          string            `json:"created,omitempty"`
	CreatorID        string            `json:"creatorId,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Name             string            `json:"name,omitempty"`
	NamespaceId      string            `json:"namespaceId,omitempty"`
	OwnerReferences  []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed          string            `json:"removed,omitempty"`
	RoleTemplateId   string            `json:"roleTemplateId,omitempty"`
	SubjectKind      string            `json:"subjectKind,omitempty"`
	SubjectName      string            `json:"subjectName,omitempty"`
	SubjectNamespace string            `json:"subjectNamespace,omitempty"`
	Uuid             string            `json:"uuid,omitempty"`
}
type ClusterRoleTemplateBindingCollection struct {
	types.Collection
	Data   []ClusterRoleTemplateBinding `json:"data,omitempty"`
	client *ClusterRoleTemplateBindingClient
}

type ClusterRoleTemplateBindingClient struct {
	apiClient *Client
}

type ClusterRoleTemplateBindingOperations interface {
	List(opts *types.ListOpts) (*ClusterRoleTemplateBindingCollection, error)
	Create(opts *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error)
	Update(existing *ClusterRoleTemplateBinding, updates interface{}) (*ClusterRoleTemplateBinding, error)
	ByID(id string) (*ClusterRoleTemplateBinding, error)
	Delete(container *ClusterRoleTemplateBinding) error
}

func newClusterRoleTemplateBindingClient(apiClient *Client) *ClusterRoleTemplateBindingClient {
	return &ClusterRoleTemplateBindingClient{
		apiClient: apiClient,
	}
}

func (c *ClusterRoleTemplateBindingClient) Create(container *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error) {
	resp := &ClusterRoleTemplateBinding{}
	err := c.apiClient.Ops.DoCreate(ClusterRoleTemplateBindingType, container, resp)
	return resp, err
}

func (c *ClusterRoleTemplateBindingClient) Update(existing *ClusterRoleTemplateBinding, updates interface{}) (*ClusterRoleTemplateBinding, error) {
	resp := &ClusterRoleTemplateBinding{}
	err := c.apiClient.Ops.DoUpdate(ClusterRoleTemplateBindingType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterRoleTemplateBindingClient) List(opts *types.ListOpts) (*ClusterRoleTemplateBindingCollection, error) {
	resp := &ClusterRoleTemplateBindingCollection{}
	err := c.apiClient.Ops.DoList(ClusterRoleTemplateBindingType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterRoleTemplateBindingCollection) Next() (*ClusterRoleTemplateBindingCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterRoleTemplateBindingCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterRoleTemplateBindingClient) ByID(id string) (*ClusterRoleTemplateBinding, error) {
	resp := &ClusterRoleTemplateBinding{}
	err := c.apiClient.Ops.DoByID(ClusterRoleTemplateBindingType, id, resp)
	return resp, err
}

func (c *ClusterRoleTemplateBindingClient) Delete(container *ClusterRoleTemplateBinding) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterRoleTemplateBindingType, &container.Resource)
}
