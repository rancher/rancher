package client

import (
	"github.com/rancher/norman/types"
)

const (
	TokenType                 = "token"
	TokenFieldAnnotations     = "annotations"
	TokenFieldAuthProvider    = "authProvider"
	TokenFieldCreated         = "created"
	TokenFieldCreatorID       = "creatorId"
	TokenFieldDescription     = "description"
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
	TokenFieldUserID          = "userId"
	TokenFieldUserPrincipal   = "userPrincipal"
	TokenFieldUuid            = "uuid"
)

type Token struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	AuthProvider    string            `json:"authProvider,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Description     string            `json:"description,omitempty"`
	Expired         bool              `json:"expired,omitempty"`
	ExpiresAt       string            `json:"expiresAt,omitempty"`
	GroupPrincipals []string          `json:"groupPrincipals,omitempty"`
	IsDerived       bool              `json:"isDerived,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	LastUpdateTime  string            `json:"lastUpdateTime,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	ProviderInfo    map[string]string `json:"providerInfo,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	TTLMillis       *int64            `json:"ttl,omitempty"`
	Token           string            `json:"token,omitempty"`
	UserID          string            `json:"userId,omitempty"`
	UserPrincipal   string            `json:"userPrincipal,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
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
	Create(opts *Token) (*Token, error)
	Update(existing *Token, updates interface{}) (*Token, error)
	ByID(id string) (*Token, error)
	Delete(container *Token) error
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

func (c *TokenClient) List(opts *types.ListOpts) (*TokenCollection, error) {
	resp := &TokenCollection{}
	err := c.apiClient.Ops.DoList(TokenType, opts, resp)
	resp.client = c
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
