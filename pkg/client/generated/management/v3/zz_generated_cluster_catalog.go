package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterCatalogType                      = "clusterCatalog"
	ClusterCatalogFieldAnnotations          = "annotations"
	ClusterCatalogFieldBranch               = "branch"
	ClusterCatalogFieldCatalogSecrets       = "catalogSecrets"
	ClusterCatalogFieldClusterID            = "clusterId"
	ClusterCatalogFieldCommit               = "commit"
	ClusterCatalogFieldConditions           = "conditions"
	ClusterCatalogFieldCreated              = "created"
	ClusterCatalogFieldCreatorID            = "creatorId"
	ClusterCatalogFieldCredentialSecret     = "credentialSecret"
	ClusterCatalogFieldDescription          = "description"
	ClusterCatalogFieldHelmVersion          = "helmVersion"
	ClusterCatalogFieldKind                 = "kind"
	ClusterCatalogFieldLabels               = "labels"
	ClusterCatalogFieldLastRefreshTimestamp = "lastRefreshTimestamp"
	ClusterCatalogFieldName                 = "name"
	ClusterCatalogFieldNamespaceId          = "namespaceId"
	ClusterCatalogFieldOwnerReferences      = "ownerReferences"
	ClusterCatalogFieldPassword             = "password"
	ClusterCatalogFieldRemoved              = "removed"
	ClusterCatalogFieldState                = "state"
	ClusterCatalogFieldTransitioning        = "transitioning"
	ClusterCatalogFieldTransitioningMessage = "transitioningMessage"
	ClusterCatalogFieldURL                  = "url"
	ClusterCatalogFieldUUID                 = "uuid"
	ClusterCatalogFieldUsername             = "username"
)

type ClusterCatalog struct {
	types.Resource
	Annotations          map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Branch               string             `json:"branch,omitempty" yaml:"branch,omitempty"`
	CatalogSecrets       *CatalogSecrets    `json:"catalogSecrets,omitempty" yaml:"catalogSecrets,omitempty"`
	ClusterID            string             `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
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
	Removed              string             `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string             `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning        string             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	URL                  string             `json:"url,omitempty" yaml:"url,omitempty"`
	UUID                 string             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Username             string             `json:"username,omitempty" yaml:"username,omitempty"`
}

type ClusterCatalogCollection struct {
	types.Collection
	Data   []ClusterCatalog `json:"data,omitempty"`
	client *ClusterCatalogClient
}

type ClusterCatalogClient struct {
	apiClient *Client
}

type ClusterCatalogOperations interface {
	List(opts *types.ListOpts) (*ClusterCatalogCollection, error)
	ListAll(opts *types.ListOpts) (*ClusterCatalogCollection, error)
	Create(opts *ClusterCatalog) (*ClusterCatalog, error)
	Update(existing *ClusterCatalog, updates interface{}) (*ClusterCatalog, error)
	Replace(existing *ClusterCatalog) (*ClusterCatalog, error)
	ByID(id string) (*ClusterCatalog, error)
	Delete(container *ClusterCatalog) error

	ActionRefresh(resource *ClusterCatalog) (*CatalogRefresh, error)

	CollectionActionRefresh(resource *ClusterCatalogCollection) (*CatalogRefresh, error)
}

func newClusterCatalogClient(apiClient *Client) *ClusterCatalogClient {
	return &ClusterCatalogClient{
		apiClient: apiClient,
	}
}

func (c *ClusterCatalogClient) Create(container *ClusterCatalog) (*ClusterCatalog, error) {
	resp := &ClusterCatalog{}
	err := c.apiClient.Ops.DoCreate(ClusterCatalogType, container, resp)
	return resp, err
}

func (c *ClusterCatalogClient) Update(existing *ClusterCatalog, updates interface{}) (*ClusterCatalog, error) {
	resp := &ClusterCatalog{}
	err := c.apiClient.Ops.DoUpdate(ClusterCatalogType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterCatalogClient) Replace(obj *ClusterCatalog) (*ClusterCatalog, error) {
	resp := &ClusterCatalog{}
	err := c.apiClient.Ops.DoReplace(ClusterCatalogType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterCatalogClient) List(opts *types.ListOpts) (*ClusterCatalogCollection, error) {
	resp := &ClusterCatalogCollection{}
	err := c.apiClient.Ops.DoList(ClusterCatalogType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ClusterCatalogClient) ListAll(opts *types.ListOpts) (*ClusterCatalogCollection, error) {
	resp := &ClusterCatalogCollection{}
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

func (cc *ClusterCatalogCollection) Next() (*ClusterCatalogCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterCatalogCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterCatalogClient) ByID(id string) (*ClusterCatalog, error) {
	resp := &ClusterCatalog{}
	err := c.apiClient.Ops.DoByID(ClusterCatalogType, id, resp)
	return resp, err
}

func (c *ClusterCatalogClient) Delete(container *ClusterCatalog) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterCatalogType, &container.Resource)
}

func (c *ClusterCatalogClient) ActionRefresh(resource *ClusterCatalog) (*CatalogRefresh, error) {
	resp := &CatalogRefresh{}
	err := c.apiClient.Ops.DoAction(ClusterCatalogType, "refresh", &resource.Resource, nil, resp)
	return resp, err
}

func (c *ClusterCatalogClient) CollectionActionRefresh(resource *ClusterCatalogCollection) (*CatalogRefresh, error) {
	resp := &CatalogRefresh{}
	err := c.apiClient.Ops.DoCollectionAction(ClusterCatalogType, "refresh", &resource.Collection, nil, resp)
	return resp, err
}
