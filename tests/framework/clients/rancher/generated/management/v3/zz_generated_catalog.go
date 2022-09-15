package client

import (
	"github.com/rancher/norman/types"
)

const (
	CatalogType                      = "catalog"
	CatalogFieldAnnotations          = "annotations"
	CatalogFieldBranch               = "branch"
	CatalogFieldCatalogSecrets       = "catalogSecrets"
	CatalogFieldCommit               = "commit"
	CatalogFieldConditions           = "conditions"
	CatalogFieldCreated              = "created"
	CatalogFieldCreatorID            = "creatorId"
	CatalogFieldCredentialSecret     = "credentialSecret"
	CatalogFieldDescription          = "description"
	CatalogFieldHelmVersion          = "helmVersion"
	CatalogFieldKind                 = "kind"
	CatalogFieldLabels               = "labels"
	CatalogFieldLastRefreshTimestamp = "lastRefreshTimestamp"
	CatalogFieldName                 = "name"
	CatalogFieldOwnerReferences      = "ownerReferences"
	CatalogFieldPassword             = "password"
	CatalogFieldRemoved              = "removed"
	CatalogFieldState                = "state"
	CatalogFieldTransitioning        = "transitioning"
	CatalogFieldTransitioningMessage = "transitioningMessage"
	CatalogFieldURL                  = "url"
	CatalogFieldUUID                 = "uuid"
	CatalogFieldUsername             = "username"
)

type Catalog struct {
	types.Resource
	Annotations          map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Branch               string             `json:"branch,omitempty" yaml:"branch,omitempty"`
	CatalogSecrets       *CatalogSecrets    `json:"catalogSecrets,omitempty" yaml:"catalogSecrets,omitempty"`
	Commit               string             `json:"commit,omitempty" yaml:"commit,omitempty"`
	Conditions           []CatalogCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created              string             `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string             `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	CredentialSecret     string             `json:"credentialSecret,omitempty" yaml:"credentialSecret,omitempty"`
	Description          string             `json:"description,omitempty" yaml:"description,omitempty"`
	HelmVersion          string             `json:"helmVersion,omitempty" yaml:"helmVersion,omitempty"`
	Kind                 string             `json:"kind,omitempty" yaml:"kind,omitempty"`
	Labels               map[string]string  `json:"labels,omitempty" yaml:"labels,omitempty"`
	LastRefreshTimestamp string             `json:"lastRefreshTimestamp,omitempty" yaml:"lastRefreshTimestamp,omitempty"`
	Name                 string             `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Password             string             `json:"password,omitempty" yaml:"password,omitempty"`
	Removed              string             `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string             `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning        string             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	URL                  string             `json:"url,omitempty" yaml:"url,omitempty"`
	UUID                 string             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Username             string             `json:"username,omitempty" yaml:"username,omitempty"`
}

type CatalogCollection struct {
	types.Collection
	Data   []Catalog `json:"data,omitempty"`
	client *CatalogClient
}

type CatalogClient struct {
	apiClient *Client
}

type CatalogOperations interface {
	List(opts *types.ListOpts) (*CatalogCollection, error)
	ListAll(opts *types.ListOpts) (*CatalogCollection, error)
	Create(opts *Catalog) (*Catalog, error)
	Update(existing *Catalog, updates interface{}) (*Catalog, error)
	Replace(existing *Catalog) (*Catalog, error)
	ByID(id string) (*Catalog, error)
	Delete(container *Catalog) error

	ActionRefresh(resource *Catalog) (*CatalogRefresh, error)

	CollectionActionRefresh(resource *CatalogCollection) (*CatalogRefresh, error)
}

func newCatalogClient(apiClient *Client) *CatalogClient {
	return &CatalogClient{
		apiClient: apiClient,
	}
}

func (c *CatalogClient) Create(container *Catalog) (*Catalog, error) {
	resp := &Catalog{}
	err := c.apiClient.Ops.DoCreate(CatalogType, container, resp)
	return resp, err
}

func (c *CatalogClient) Update(existing *Catalog, updates interface{}) (*Catalog, error) {
	resp := &Catalog{}
	err := c.apiClient.Ops.DoUpdate(CatalogType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CatalogClient) Replace(obj *Catalog) (*Catalog, error) {
	resp := &Catalog{}
	err := c.apiClient.Ops.DoReplace(CatalogType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *CatalogClient) List(opts *types.ListOpts) (*CatalogCollection, error) {
	resp := &CatalogCollection{}
	err := c.apiClient.Ops.DoList(CatalogType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *CatalogClient) ListAll(opts *types.ListOpts) (*CatalogCollection, error) {
	resp := &CatalogCollection{}
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

func (cc *CatalogCollection) Next() (*CatalogCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CatalogCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CatalogClient) ByID(id string) (*Catalog, error) {
	resp := &Catalog{}
	err := c.apiClient.Ops.DoByID(CatalogType, id, resp)
	return resp, err
}

func (c *CatalogClient) Delete(container *Catalog) error {
	return c.apiClient.Ops.DoResourceDelete(CatalogType, &container.Resource)
}

func (c *CatalogClient) ActionRefresh(resource *Catalog) (*CatalogRefresh, error) {
	resp := &CatalogRefresh{}
	err := c.apiClient.Ops.DoAction(CatalogType, "refresh", &resource.Resource, nil, resp)
	return resp, err
}

func (c *CatalogClient) CollectionActionRefresh(resource *CatalogCollection) (*CatalogRefresh, error) {
	resp := &CatalogRefresh{}
	err := c.apiClient.Ops.DoCollectionAction(CatalogType, "refresh", &resource.Collection, nil, resp)
	return resp, err
}
