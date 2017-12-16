package client

import (
	"github.com/rancher/norman/types"
)

const (
	GroupMemberType                 = "groupMember"
	GroupMemberFieldAnnotations     = "annotations"
	GroupMemberFieldCreated         = "created"
	GroupMemberFieldFinalizers      = "finalizers"
	GroupMemberFieldGroupId         = "groupId"
	GroupMemberFieldLabels          = "labels"
	GroupMemberFieldName            = "name"
	GroupMemberFieldOwnerReferences = "ownerReferences"
	GroupMemberFieldPrincipalID     = "principalId"
	GroupMemberFieldRemoved         = "removed"
	GroupMemberFieldResourcePath    = "resourcePath"
	GroupMemberFieldUuid            = "uuid"
)

type GroupMember struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	GroupId         string            `json:"groupId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	PrincipalID     string            `json:"principalId,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	ResourcePath    string            `json:"resourcePath,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type GroupMemberCollection struct {
	types.Collection
	Data   []GroupMember `json:"data,omitempty"`
	client *GroupMemberClient
}

type GroupMemberClient struct {
	apiClient *Client
}

type GroupMemberOperations interface {
	List(opts *types.ListOpts) (*GroupMemberCollection, error)
	Create(opts *GroupMember) (*GroupMember, error)
	Update(existing *GroupMember, updates interface{}) (*GroupMember, error)
	ByID(id string) (*GroupMember, error)
	Delete(container *GroupMember) error
}

func newGroupMemberClient(apiClient *Client) *GroupMemberClient {
	return &GroupMemberClient{
		apiClient: apiClient,
	}
}

func (c *GroupMemberClient) Create(container *GroupMember) (*GroupMember, error) {
	resp := &GroupMember{}
	err := c.apiClient.Ops.DoCreate(GroupMemberType, container, resp)
	return resp, err
}

func (c *GroupMemberClient) Update(existing *GroupMember, updates interface{}) (*GroupMember, error) {
	resp := &GroupMember{}
	err := c.apiClient.Ops.DoUpdate(GroupMemberType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *GroupMemberClient) List(opts *types.ListOpts) (*GroupMemberCollection, error) {
	resp := &GroupMemberCollection{}
	err := c.apiClient.Ops.DoList(GroupMemberType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *GroupMemberCollection) Next() (*GroupMemberCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &GroupMemberCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *GroupMemberClient) ByID(id string) (*GroupMember, error) {
	resp := &GroupMember{}
	err := c.apiClient.Ops.DoByID(GroupMemberType, id, resp)
	return resp, err
}

func (c *GroupMemberClient) Delete(container *GroupMember) error {
	return c.apiClient.Ops.DoResourceDelete(GroupMemberType, &container.Resource)
}
