package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectCatalogType                      = "projectCatalog"
	ProjectCatalogFieldAnnotations          = "annotations"
	ProjectCatalogFieldBranch               = "branch"
	ProjectCatalogFieldCatalogSecrets       = "catalogSecrets"
	ProjectCatalogFieldCommit               = "commit"
	ProjectCatalogFieldConditions           = "conditions"
	ProjectCatalogFieldCreated              = "created"
	ProjectCatalogFieldCreatorID            = "creatorId"
	ProjectCatalogFieldCredentialSecret     = "credentialSecret"
	ProjectCatalogFieldDescription          = "description"
	ProjectCatalogFieldHelmVersion          = "helmVersion"
	ProjectCatalogFieldKind                 = "kind"
	ProjectCatalogFieldLabels               = "labels"
	ProjectCatalogFieldLastRefreshTimestamp = "lastRefreshTimestamp"
	ProjectCatalogFieldName                 = "name"
	ProjectCatalogFieldNamespaceId          = "namespaceId"
	ProjectCatalogFieldOwnerReferences      = "ownerReferences"
	ProjectCatalogFieldPassword             = "password"
	ProjectCatalogFieldProjectID            = "projectId"
	ProjectCatalogFieldRemoved              = "removed"
	ProjectCatalogFieldState                = "state"
	ProjectCatalogFieldTransitioning        = "transitioning"
	ProjectCatalogFieldTransitioningMessage = "transitioningMessage"
	ProjectCatalogFieldURL                  = "url"
	ProjectCatalogFieldUUID                 = "uuid"
	ProjectCatalogFieldUsername             = "username"
)

type ProjectCatalog struct {
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
	NamespaceId          string             `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Password             string             `json:"password,omitempty" yaml:"password,omitempty"`
	ProjectID            string             `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed              string             `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string             `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning        string             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	URL                  string             `json:"url,omitempty" yaml:"url,omitempty"`
	UUID                 string             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Username             string             `json:"username,omitempty" yaml:"username,omitempty"`
}

type ProjectCatalogCollection struct {
	types.Collection
	Data   []ProjectCatalog `json:"data,omitempty"`
	client *ProjectCatalogClient
}

type ProjectCatalogClient struct {
	apiClient *Client
}

type ProjectCatalogOperations interface {
	List(opts *types.ListOpts) (*ProjectCatalogCollection, error)
	ListAll(opts *types.ListOpts) (*ProjectCatalogCollection, error)
	Create(opts *ProjectCatalog) (*ProjectCatalog, error)
	Update(existing *ProjectCatalog, updates interface{}) (*ProjectCatalog, error)
	Replace(existing *ProjectCatalog) (*ProjectCatalog, error)
	ByID(id string) (*ProjectCatalog, error)
	Delete(container *ProjectCatalog) error

	ActionRefresh(resource *ProjectCatalog) (*CatalogRefresh, error)

	CollectionActionRefresh(resource *ProjectCatalogCollection) (*CatalogRefresh, error)
}

func newProjectCatalogClient(apiClient *Client) *ProjectCatalogClient {
	return &ProjectCatalogClient{
		apiClient: apiClient,
	}
}

func (c *ProjectCatalogClient) Create(container *ProjectCatalog) (*ProjectCatalog, error) {
	resp := &ProjectCatalog{}
	err := c.apiClient.Ops.DoCreate(ProjectCatalogType, container, resp)
	return resp, err
}

func (c *ProjectCatalogClient) Update(existing *ProjectCatalog, updates interface{}) (*ProjectCatalog, error) {
	resp := &ProjectCatalog{}
	err := c.apiClient.Ops.DoUpdate(ProjectCatalogType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectCatalogClient) Replace(obj *ProjectCatalog) (*ProjectCatalog, error) {
	resp := &ProjectCatalog{}
	err := c.apiClient.Ops.DoReplace(ProjectCatalogType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ProjectCatalogClient) List(opts *types.ListOpts) (*ProjectCatalogCollection, error) {
	resp := &ProjectCatalogCollection{}
	err := c.apiClient.Ops.DoList(ProjectCatalogType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ProjectCatalogClient) ListAll(opts *types.ListOpts) (*ProjectCatalogCollection, error) {
	resp := &ProjectCatalogCollection{}
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

func (cc *ProjectCatalogCollection) Next() (*ProjectCatalogCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectCatalogCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectCatalogClient) ByID(id string) (*ProjectCatalog, error) {
	resp := &ProjectCatalog{}
	err := c.apiClient.Ops.DoByID(ProjectCatalogType, id, resp)
	return resp, err
}

func (c *ProjectCatalogClient) Delete(container *ProjectCatalog) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectCatalogType, &container.Resource)
}

func (c *ProjectCatalogClient) ActionRefresh(resource *ProjectCatalog) (*CatalogRefresh, error) {
	resp := &CatalogRefresh{}
	err := c.apiClient.Ops.DoAction(ProjectCatalogType, "refresh", &resource.Resource, nil, resp)
	return resp, err
}

func (c *ProjectCatalogClient) CollectionActionRefresh(resource *ProjectCatalogCollection) (*CatalogRefresh, error) {
	resp := &CatalogRefresh{}
	err := c.apiClient.Ops.DoCollectionAction(ProjectCatalogType, "refresh", &resource.Collection, nil, resp)
	return resp, err
}
