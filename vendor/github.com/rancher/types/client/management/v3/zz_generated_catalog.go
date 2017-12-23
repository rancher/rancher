package client

import (
	"github.com/rancher/norman/types"
)

const (
	CatalogType                      = "catalog"
	CatalogFieldAnnotations          = "annotations"
	CatalogFieldBranch               = "branch"
	CatalogFieldCatalogKind          = "catalogKind"
	CatalogFieldCreated              = "created"
	CatalogFieldFinalizers           = "finalizers"
	CatalogFieldLabels               = "labels"
	CatalogFieldName                 = "name"
	CatalogFieldOwnerReferences      = "ownerReferences"
	CatalogFieldRemoved              = "removed"
	CatalogFieldState                = "state"
	CatalogFieldStatus               = "status"
	CatalogFieldTransitioning        = "transitioning"
	CatalogFieldTransitioningMessage = "transitioningMessage"
	CatalogFieldURL                  = "url"
	CatalogFieldUuid                 = "uuid"
)

type Catalog struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty"`
	Branch               string            `json:"branch,omitempty"`
	CatalogKind          string            `json:"catalogKind,omitempty"`
	Created              string            `json:"created,omitempty"`
	Finalizers           []string          `json:"finalizers,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Name                 string            `json:"name,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed              string            `json:"removed,omitempty"`
	State                string            `json:"state,omitempty"`
	Status               *CatalogStatus    `json:"status,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty"`
	URL                  string            `json:"url,omitempty"`
	Uuid                 string            `json:"uuid,omitempty"`
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
	Create(opts *Catalog) (*Catalog, error)
	Update(existing *Catalog, updates interface{}) (*Catalog, error)
	ByID(id string) (*Catalog, error)
	Delete(container *Catalog) error
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

func (c *CatalogClient) List(opts *types.ListOpts) (*CatalogCollection, error) {
	resp := &CatalogCollection{}
	err := c.apiClient.Ops.DoList(CatalogType, opts, resp)
	resp.client = c
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
