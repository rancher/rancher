package client

import (
	"github.com/rancher/norman/types"
)

const (
	PrincipalType                 = "principal"
	PrincipalFieldAnnotations     = "annotations"
	PrincipalFieldCreated         = "created"
	PrincipalFieldCreatorID       = "creatorId"
	PrincipalFieldExtraInfo       = "extraInfo"
	PrincipalFieldLabels          = "labels"
	PrincipalFieldLoginName       = "loginName"
	PrincipalFieldMe              = "me"
	PrincipalFieldMemberOf        = "memberOf"
	PrincipalFieldName            = "name"
	PrincipalFieldOwnerReferences = "ownerReferences"
	PrincipalFieldPrincipalType   = "principalType"
	PrincipalFieldProfilePicture  = "profilePicture"
	PrincipalFieldProfileURL      = "profileURL"
	PrincipalFieldProvider        = "provider"
	PrincipalFieldRemoved         = "removed"
	PrincipalFieldUUID            = "uuid"
)

type Principal struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ExtraInfo       map[string]string `json:"extraInfo,omitempty" yaml:"extraInfo,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LoginName       string            `json:"loginName,omitempty" yaml:"loginName,omitempty"`
	Me              bool              `json:"me,omitempty" yaml:"me,omitempty"`
	MemberOf        bool              `json:"memberOf,omitempty" yaml:"memberOf,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PrincipalType   string            `json:"principalType,omitempty" yaml:"principalType,omitempty"`
	ProfilePicture  string            `json:"profilePicture,omitempty" yaml:"profilePicture,omitempty"`
	ProfileURL      string            `json:"profileURL,omitempty" yaml:"profileURL,omitempty"`
	Provider        string            `json:"provider,omitempty" yaml:"provider,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
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
	ListAll(opts *types.ListOpts) (*PrincipalCollection, error)
	Create(opts *Principal) (*Principal, error)
	Update(existing *Principal, updates interface{}) (*Principal, error)
	Replace(existing *Principal) (*Principal, error)
	ByID(id string) (*Principal, error)
	Delete(container *Principal) error

	CollectionActionSearch(resource *PrincipalCollection, input *SearchPrincipalsInput) (*PrincipalCollection, error)
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

func (c *PrincipalClient) Replace(obj *Principal) (*Principal, error) {
	resp := &Principal{}
	err := c.apiClient.Ops.DoReplace(PrincipalType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PrincipalClient) List(opts *types.ListOpts) (*PrincipalCollection, error) {
	resp := &PrincipalCollection{}
	err := c.apiClient.Ops.DoList(PrincipalType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *PrincipalClient) ListAll(opts *types.ListOpts) (*PrincipalCollection, error) {
	resp := &PrincipalCollection{}
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

func (c *PrincipalClient) CollectionActionSearch(resource *PrincipalCollection, input *SearchPrincipalsInput) (*PrincipalCollection, error) {
	resp := &PrincipalCollection{}
	err := c.apiClient.Ops.DoCollectionAction(PrincipalType, "search", &resource.Collection, input, resp)
	return resp, err
}
