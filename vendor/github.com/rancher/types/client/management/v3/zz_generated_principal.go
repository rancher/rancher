package client

import (
	"github.com/rancher/norman/types"
)

const (
	PrincipalType                 = "principal"
	PrincipalField                = "creatorId"
	PrincipalFieldAnnotations     = "annotations"
	PrincipalFieldCreated         = "created"
	PrincipalFieldDisplayName     = "displayName"
	PrincipalFieldExtraInfo       = "extraInfo"
	PrincipalFieldFinalizers      = "finalizers"
	PrincipalFieldLabels          = "labels"
	PrincipalFieldLoginName       = "loginName"
	PrincipalFieldMe              = "me"
	PrincipalFieldMemberOf        = "memberOf"
	PrincipalFieldName            = "name"
	PrincipalFieldOwnerReferences = "ownerReferences"
	PrincipalFieldProfilePicture  = "profilePicture"
	PrincipalFieldProfileURL      = "profileURL"
	PrincipalFieldRemoved         = "removed"
	PrincipalFieldUuid            = "uuid"
)

type Principal struct {
	types.Resource
	string          `json:"creatorId,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	DisplayName     string            `json:"displayName,omitempty"`
	ExtraInfo       map[string]string `json:"extraInfo,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	LoginName       string            `json:"loginName,omitempty"`
	Me              *bool             `json:"me,omitempty"`
	MemberOf        *bool             `json:"memberOf,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	ProfilePicture  string            `json:"profilePicture,omitempty"`
	ProfileURL      string            `json:"profileURL,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type PrincipalCollection struct {
	types.Collection
	Data   []Principal `json:"data,omitempty"`
	client *PrincipalClient
}

type PrincipalClient struct {
	apiClient *Client
}

type PrincipalOperations interface {
	List(opts *types.ListOpts) (*PrincipalCollection, error)
	Create(opts *Principal) (*Principal, error)
	Update(existing *Principal, updates interface{}) (*Principal, error)
	ByID(id string) (*Principal, error)
	Delete(container *Principal) error
}

func newPrincipalClient(apiClient *Client) *PrincipalClient {
	return &PrincipalClient{
		apiClient: apiClient,
	}
}

func (c *PrincipalClient) Create(container *Principal) (*Principal, error) {
	resp := &Principal{}
	err := c.apiClient.Ops.DoCreate(PrincipalType, container, resp)
	return resp, err
}

func (c *PrincipalClient) Update(existing *Principal, updates interface{}) (*Principal, error) {
	resp := &Principal{}
	err := c.apiClient.Ops.DoUpdate(PrincipalType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PrincipalClient) List(opts *types.ListOpts) (*PrincipalCollection, error) {
	resp := &PrincipalCollection{}
	err := c.apiClient.Ops.DoList(PrincipalType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *PrincipalCollection) Next() (*PrincipalCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PrincipalCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PrincipalClient) ByID(id string) (*Principal, error) {
	resp := &Principal{}
	err := c.apiClient.Ops.DoByID(PrincipalType, id, resp)
	return resp, err
}

func (c *PrincipalClient) Delete(container *Principal) error {
	return c.apiClient.Ops.DoResourceDelete(PrincipalType, &container.Resource)
}
