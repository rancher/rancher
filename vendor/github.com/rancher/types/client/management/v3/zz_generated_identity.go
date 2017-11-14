package client

import (
	"github.com/rancher/norman/types"
)

const (
	IdentityType                 = "identity"
	IdentityFieldAnnotations     = "annotations"
	IdentityFieldCreated         = "created"
	IdentityFieldDisplayName     = "displayName"
	IdentityFieldExtraInfo       = "extraInfo"
	IdentityFieldFinalizers      = "finalizers"
	IdentityFieldLabels          = "labels"
	IdentityFieldLoginName       = "loginName"
	IdentityFieldMe              = "me"
	IdentityFieldMemberOf        = "memberOf"
	IdentityFieldName            = "name"
	IdentityFieldOwnerReferences = "ownerReferences"
	IdentityFieldProfilePicture  = "profilePicture"
	IdentityFieldProfileURL      = "profileURL"
	IdentityFieldRemoved         = "removed"
	IdentityFieldResourcePath    = "resourcePath"
	IdentityFieldUuid            = "uuid"
)

type Identity struct {
	types.Resource
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
	ResourcePath    string            `json:"resourcePath,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type IdentityCollection struct {
	types.Collection
	Data   []Identity `json:"data,omitempty"`
	client *IdentityClient
}

type IdentityClient struct {
	apiClient *Client
}

type IdentityOperations interface {
	List(opts *types.ListOpts) (*IdentityCollection, error)
	Create(opts *Identity) (*Identity, error)
	Update(existing *Identity, updates interface{}) (*Identity, error)
	ByID(id string) (*Identity, error)
	Delete(container *Identity) error
}

func newIdentityClient(apiClient *Client) *IdentityClient {
	return &IdentityClient{
		apiClient: apiClient,
	}
}

func (c *IdentityClient) Create(container *Identity) (*Identity, error) {
	resp := &Identity{}
	err := c.apiClient.Ops.DoCreate(IdentityType, container, resp)
	return resp, err
}

func (c *IdentityClient) Update(existing *Identity, updates interface{}) (*Identity, error) {
	resp := &Identity{}
	err := c.apiClient.Ops.DoUpdate(IdentityType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *IdentityClient) List(opts *types.ListOpts) (*IdentityCollection, error) {
	resp := &IdentityCollection{}
	err := c.apiClient.Ops.DoList(IdentityType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *IdentityCollection) Next() (*IdentityCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &IdentityCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *IdentityClient) ByID(id string) (*Identity, error) {
	resp := &Identity{}
	err := c.apiClient.Ops.DoByID(IdentityType, id, resp)
	return resp, err
}

func (c *IdentityClient) Delete(container *Identity) error {
	return c.apiClient.Ops.DoResourceDelete(IdentityType, &container.Resource)
}
