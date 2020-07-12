package client

import (
	"github.com/rancher/norman/types"
)

const (
	TokenType                 = "token"
	TokenFieldAnnotations     = "annotations"
	TokenFieldAuthProvider    = "authProvider"
	TokenFieldClusterID       = "clusterId"
	TokenFieldCreated         = "created"
	TokenFieldCreatorID       = "creatorId"
	TokenFieldCurrent         = "current"
	TokenFieldDescription     = "description"
	TokenFieldEnabled         = "enabled"
	TokenFieldExpired         = "expired"
	TokenFieldExpiresAt       = "expiresAt"
	TokenFieldGroupPrincipals = "groupPrincipals"
	TokenFieldIsDerived       = "isDerived"
	TokenFieldLabels          = "labels"
	TokenFieldLastUpdateTime  = "lastUpdateTime"
	TokenFieldName            = "name"
	TokenFieldOwnerReferences = "ownerReferences"
	TokenFieldProviderInfo    = "providerInfo"
	TokenFieldRemoved         = "removed"
	TokenFieldTTLMillis       = "ttl"
	TokenFieldToken           = "token"
	TokenFieldUUID            = "uuid"
	TokenFieldUserID          = "userId"
	TokenFieldUserPrincipal   = "userPrincipal"
)

type Token struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthProvider    string            `json:"authProvider,omitempty" yaml:"authProvider,omitempty"`
	ClusterID       string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Current         bool              `json:"current,omitempty" yaml:"current,omitempty"`
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled         *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Expired         bool              `json:"expired,omitempty" yaml:"expired,omitempty"`
	ExpiresAt       string            `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	GroupPrincipals []string          `json:"groupPrincipals,omitempty" yaml:"groupPrincipals,omitempty"`
	IsDerived       bool              `json:"isDerived,omitempty" yaml:"isDerived,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LastUpdateTime  string            `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProviderInfo    map[string]string `json:"providerInfo,omitempty" yaml:"providerInfo,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	TTLMillis       int64             `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Token           string            `json:"token,omitempty" yaml:"token,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserID          string            `json:"userId,omitempty" yaml:"userId,omitempty"`
	UserPrincipal   string            `json:"userPrincipal,omitempty" yaml:"userPrincipal,omitempty"`
}

type TokenCollection struct {
	types.Collection
	Data   []Token `json:"data,omitempty"`
	client *TokenClient
}

type TokenClient struct {
	apiClient *Client
}

type TokenOperations interface {
	List(opts *types.ListOpts) (*TokenCollection, error)
	ListAll(opts *types.ListOpts) (*TokenCollection, error)
	Create(opts *Token) (*Token, error)
	Update(existing *Token, updates interface{}) (*Token, error)
	Replace(existing *Token) (*Token, error)
	ByID(id string) (*Token, error)
	Delete(container *Token) error

	CollectionActionLogout(resource *TokenCollection) error
}

func newTokenClient(apiClient *Client) *TokenClient {
	return &TokenClient{
		apiClient: apiClient,
	}
}

func (c *TokenClient) Create(container *Token) (*Token, error) {
	resp := &Token{}
	err := c.apiClient.Ops.DoCreate(TokenType, container, resp)
	return resp, err
}

func (c *TokenClient) Update(existing *Token, updates interface{}) (*Token, error) {
	resp := &Token{}
	err := c.apiClient.Ops.DoUpdate(TokenType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *TokenClient) Replace(obj *Token) (*Token, error) {
	resp := &Token{}
	err := c.apiClient.Ops.DoReplace(TokenType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *TokenClient) List(opts *types.ListOpts) (*TokenCollection, error) {
	resp := &TokenCollection{}
	err := c.apiClient.Ops.DoList(TokenType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *TokenClient) ListAll(opts *types.ListOpts) (*TokenCollection, error) {
	resp := &TokenCollection{}
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

func (cc *TokenCollection) Next() (*TokenCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &TokenCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *TokenClient) ByID(id string) (*Token, error) {
	resp := &Token{}
	err := c.apiClient.Ops.DoByID(TokenType, id, resp)
	return resp, err
}

func (c *TokenClient) Delete(container *Token) error {
	return c.apiClient.Ops.DoResourceDelete(TokenType, &container.Resource)
}

func (c *TokenClient) CollectionActionLogout(resource *TokenCollection) error {
	err := c.apiClient.Ops.DoCollectionAction(TokenType, "logout", &resource.Collection, nil, nil)
	return err
}
