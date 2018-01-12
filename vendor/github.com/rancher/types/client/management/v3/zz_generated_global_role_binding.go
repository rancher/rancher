package client

import (
	"github.com/rancher/norman/types"
)

const (
	GlobalRoleBindingType                 = "globalRoleBinding"
	GlobalRoleBindingFieldAnnotations     = "annotations"
	GlobalRoleBindingFieldCreated         = "created"
	GlobalRoleBindingFieldCreatorID       = "creatorId"
	GlobalRoleBindingFieldGlobalRoleId    = "globalRoleId"
	GlobalRoleBindingFieldLabels          = "labels"
	GlobalRoleBindingFieldName            = "name"
	GlobalRoleBindingFieldOwnerReferences = "ownerReferences"
	GlobalRoleBindingFieldRemoved         = "removed"
	GlobalRoleBindingFieldSubjectKind     = "subjectKind"
	GlobalRoleBindingFieldSubjectName     = "subjectName"
	GlobalRoleBindingFieldUuid            = "uuid"
)

type GlobalRoleBinding struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	GlobalRoleId    string            `json:"globalRoleId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	SubjectKind     string            `json:"subjectKind,omitempty"`
	SubjectName     string            `json:"subjectName,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type GlobalRoleBindingCollection struct {
	types.Collection
	Data   []GlobalRoleBinding `json:"data,omitempty"`
	client *GlobalRoleBindingClient
}

type GlobalRoleBindingClient struct {
	apiClient *Client
}

type GlobalRoleBindingOperations interface {
	List(opts *types.ListOpts) (*GlobalRoleBindingCollection, error)
	Create(opts *GlobalRoleBinding) (*GlobalRoleBinding, error)
	Update(existing *GlobalRoleBinding, updates interface{}) (*GlobalRoleBinding, error)
	ByID(id string) (*GlobalRoleBinding, error)
	Delete(container *GlobalRoleBinding) error
}

func newGlobalRoleBindingClient(apiClient *Client) *GlobalRoleBindingClient {
	return &GlobalRoleBindingClient{
		apiClient: apiClient,
	}
}

func (c *GlobalRoleBindingClient) Create(container *GlobalRoleBinding) (*GlobalRoleBinding, error) {
	resp := &GlobalRoleBinding{}
	err := c.apiClient.Ops.DoCreate(GlobalRoleBindingType, container, resp)
	return resp, err
}

func (c *GlobalRoleBindingClient) Update(existing *GlobalRoleBinding, updates interface{}) (*GlobalRoleBinding, error) {
	resp := &GlobalRoleBinding{}
	err := c.apiClient.Ops.DoUpdate(GlobalRoleBindingType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GlobalRoleBindingClient) List(opts *types.ListOpts) (*GlobalRoleBindingCollection, error) {
	resp := &GlobalRoleBindingCollection{}
	err := c.apiClient.Ops.DoList(GlobalRoleBindingType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *GlobalRoleBindingCollection) Next() (*GlobalRoleBindingCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GlobalRoleBindingCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GlobalRoleBindingClient) ByID(id string) (*GlobalRoleBinding, error) {
	resp := &GlobalRoleBinding{}
	err := c.apiClient.Ops.DoByID(GlobalRoleBindingType, id, resp)
	return resp, err
}

func (c *GlobalRoleBindingClient) Delete(container *GlobalRoleBinding) error {
	return c.apiClient.Ops.DoResourceDelete(GlobalRoleBindingType, &container.Resource)
}
