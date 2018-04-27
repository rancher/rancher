package client

import (
	"github.com/rancher/norman/types"
)

const (
	UserType                    = "user"
	UserFieldAnnotations        = "annotations"
	UserFieldCreated            = "created"
	UserFieldCreatorID          = "creatorId"
	UserFieldDescription        = "description"
	UserFieldLabels             = "labels"
	UserFieldMe                 = "me"
	UserFieldMustChangePassword = "mustChangePassword"
	UserFieldName               = "name"
	UserFieldOwnerReferences    = "ownerReferences"
	UserFieldPassword           = "password"
	UserFieldPrincipalIDs       = "principalIds"
	UserFieldRemoved            = "removed"
	UserFieldUsername           = "username"
	UserFieldUuid               = "uuid"
)

type User struct {
	types.Resource
	Annotations        map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created            string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID          string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description        string            `json:"description,omitempty" yaml:"description,omitempty"`
	Labels             map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Me                 bool              `json:"me,omitempty" yaml:"me,omitempty"`
	MustChangePassword bool              `json:"mustChangePassword,omitempty" yaml:"mustChangePassword,omitempty"`
	Name               string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences    []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Password           string            `json:"password,omitempty" yaml:"password,omitempty"`
	PrincipalIDs       []string          `json:"principalIds,omitempty" yaml:"principalIds,omitempty"`
	Removed            string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Username           string            `json:"username,omitempty" yaml:"username,omitempty"`
	Uuid               string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type UserCollection struct {
	types.Collection
	Data   []User `json:"data,omitempty"`
	client *UserClient
}

type UserClient struct {
	apiClient *Client
}

type UserOperations interface {
	List(opts *types.ListOpts) (*UserCollection, error)
	Create(opts *User) (*User, error)
	Update(existing *User, updates interface{}) (*User, error)
	ByID(id string) (*User, error)
	Delete(container *User) error

	ActionSetpassword(resource *User, input *SetPasswordInput) (*User, error)

	CollectionActionChangepassword(resource *UserCollection, input *ChangePasswordInput) error
}

func newUserClient(apiClient *Client) *UserClient {
	return &UserClient{
		apiClient: apiClient,
	}
}

func (c *UserClient) Create(container *User) (*User, error) {
	resp := &User{}
	err := c.apiClient.Ops.DoCreate(UserType, container, resp)
	return resp, err
}

func (c *UserClient) Update(existing *User, updates interface{}) (*User, error) {
	resp := &User{}
	err := c.apiClient.Ops.DoUpdate(UserType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *UserClient) List(opts *types.ListOpts) (*UserCollection, error) {
	resp := &UserCollection{}
	err := c.apiClient.Ops.DoList(UserType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *UserCollection) Next() (*UserCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &UserCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *UserClient) ByID(id string) (*User, error) {
	resp := &User{}
	err := c.apiClient.Ops.DoByID(UserType, id, resp)
	return resp, err
}

func (c *UserClient) Delete(container *User) error {
	return c.apiClient.Ops.DoResourceDelete(UserType, &container.Resource)
}

func (c *UserClient) ActionSetpassword(resource *User, input *SetPasswordInput) (*User, error) {
	resp := &User{}
	err := c.apiClient.Ops.DoAction(UserType, "setpassword", &resource.Resource, input, resp)
	return resp, err
}

func (c *UserClient) CollectionActionChangepassword(resource *UserCollection, input *ChangePasswordInput) error {
	err := c.apiClient.Ops.DoCollectionAction(UserType, "changepassword", &resource.Collection, input, nil)
	return err
}
