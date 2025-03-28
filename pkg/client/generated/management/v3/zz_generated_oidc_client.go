package client

import (
	"github.com/rancher/norman/types"
)

const (
	OIDCClientType                               = "oidcClient"
	OIDCClientFieldAnnotations                   = "annotations"
	OIDCClientFieldCreated                       = "created"
	OIDCClientFieldCreatorID                     = "creatorId"
	OIDCClientFieldDescription                   = "description"
	OIDCClientFieldLabels                        = "labels"
	OIDCClientFieldName                          = "name"
	OIDCClientFieldOwnerReferences               = "ownerReferences"
	OIDCClientFieldRedirectURIs                  = "redirectURIs"
	OIDCClientFieldRefreshTokenExpirationSeconds = "refreshTokenExpirationSeconds"
	OIDCClientFieldRemoved                       = "removed"
	OIDCClientFieldState                         = "state"
	OIDCClientFieldStatus                        = "status"
	OIDCClientFieldTokenExpirationSeconds        = "tokenExpirationSeconds"
	OIDCClientFieldTransitioning                 = "transitioning"
	OIDCClientFieldTransitioningMessage          = "transitioningMessage"
	OIDCClientFieldUUID                          = "uuid"
)

type OIDCClient struct {
	types.Resource
	Annotations                   map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created                       string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description                   string            `json:"description,omitempty" yaml:"description,omitempty"`
	Labels                        map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                          string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences               []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RedirectURIs                  []string          `json:"redirectURIs,omitempty" yaml:"redirectURIs,omitempty"`
	RefreshTokenExpirationSeconds int64             `json:"refreshTokenExpirationSeconds,omitempty" yaml:"refreshTokenExpirationSeconds,omitempty"`
	Removed                       string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                         string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status                        OIDCClientStatus  `json:"status,omitempty" yaml:"status,omitempty"`
	TokenExpirationSeconds        int64             `json:"tokenExpirationSeconds,omitempty" yaml:"tokenExpirationSeconds,omitempty"`
	Transitioning                 string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage          string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                          string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type OIDCClientCollection struct {
	types.Collection
	Data   []OIDCClient `json:"data,omitempty"`
	client *OIDCClientClient
}

type OIDCClientClient struct {
	apiClient *Client
}

type OIDCClientOperations interface {
	List(opts *types.ListOpts) (*OIDCClientCollection, error)
	ListAll(opts *types.ListOpts) (*OIDCClientCollection, error)
	Create(opts *OIDCClient) (*OIDCClient, error)
	Update(existing *OIDCClient, updates interface{}) (*OIDCClient, error)
	Replace(existing *OIDCClient) (*OIDCClient, error)
	ByID(id string) (*OIDCClient, error)
	Delete(container *OIDCClient) error
}

func newOIDCClientClient(apiClient *Client) *OIDCClientClient {
	return &OIDCClientClient{
		apiClient: apiClient,
	}
}

func (c *OIDCClientClient) Create(container *OIDCClient) (*OIDCClient, error) {
	resp := &OIDCClient{}
	err := c.apiClient.Ops.DoCreate(OIDCClientType, container, resp)
	return resp, err
}

func (c *OIDCClientClient) Update(existing *OIDCClient, updates interface{}) (*OIDCClient, error) {
	resp := &OIDCClient{}
	err := c.apiClient.Ops.DoUpdate(OIDCClientType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *OIDCClientClient) Replace(obj *OIDCClient) (*OIDCClient, error) {
	resp := &OIDCClient{}
	err := c.apiClient.Ops.DoReplace(OIDCClientType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *OIDCClientClient) List(opts *types.ListOpts) (*OIDCClientCollection, error) {
	resp := &OIDCClientCollection{}
	err := c.apiClient.Ops.DoList(OIDCClientType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *OIDCClientClient) ListAll(opts *types.ListOpts) (*OIDCClientCollection, error) {
	resp := &OIDCClientCollection{}
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

func (cc *OIDCClientCollection) Next() (*OIDCClientCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &OIDCClientCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *OIDCClientClient) ByID(id string) (*OIDCClient, error) {
	resp := &OIDCClient{}
	err := c.apiClient.Ops.DoByID(OIDCClientType, id, resp)
	return resp, err
}

func (c *OIDCClientClient) Delete(container *OIDCClient) error {
	return c.apiClient.Ops.DoResourceDelete(OIDCClientType, &container.Resource)
}
