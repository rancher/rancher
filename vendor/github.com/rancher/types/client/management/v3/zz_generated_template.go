package client

import (
	"github.com/rancher/norman/types"
)

const (
	TemplateType                          = "template"
	TemplateFieldAnnotations              = "annotations"
	TemplateFieldCatalogID                = "catalogId"
	TemplateFieldCategories               = "categories"
	TemplateFieldCategory                 = "category"
	TemplateFieldClusterCatalogID         = "clusterCatalogID"
	TemplateFieldClusterID                = "clusterID"
	TemplateFieldCreated                  = "created"
	TemplateFieldCreatorID                = "creatorId"
	TemplateFieldDefaultTemplateVersionID = "defaultTemplateVersionId"
	TemplateFieldDefaultVersion           = "defaultVersion"
	TemplateFieldDescription              = "description"
	TemplateFieldFolderName               = "folderName"
	TemplateFieldIcon                     = "icon"
	TemplateFieldIconFilename             = "iconFilename"
	TemplateFieldLabels                   = "labels"
	TemplateFieldMaintainer               = "maintainer"
	TemplateFieldName                     = "name"
	TemplateFieldOwnerReferences          = "ownerReferences"
	TemplateFieldPath                     = "path"
	TemplateFieldProjectCatalogID         = "projectCatalogID"
	TemplateFieldProjectID                = "projectID"
	TemplateFieldProjectURL               = "projectURL"
	TemplateFieldReadme                   = "readme"
	TemplateFieldRemoved                  = "removed"
	TemplateFieldState                    = "state"
	TemplateFieldStatus                   = "status"
	TemplateFieldTransitioning            = "transitioning"
	TemplateFieldTransitioningMessage     = "transitioningMessage"
	TemplateFieldUUID                     = "uuid"
	TemplateFieldUpgradeFrom              = "upgradeFrom"
	TemplateFieldVersionLinks             = "versionLinks"
	TemplateFieldVersions                 = "versions"
)

type Template struct {
	types.Resource
	Annotations              map[string]string     `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	CatalogID                string                `json:"catalogId,omitempty" yaml:"catalogId,omitempty"`
	Categories               []string              `json:"categories,omitempty" yaml:"categories,omitempty"`
	Category                 string                `json:"category,omitempty" yaml:"category,omitempty"`
	ClusterCatalogID         string                `json:"clusterCatalogID,omitempty" yaml:"clusterCatalogID,omitempty"`
	ClusterID                string                `json:"clusterID,omitempty" yaml:"clusterID,omitempty"`
	Created                  string                `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                string                `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DefaultTemplateVersionID string                `json:"defaultTemplateVersionId,omitempty" yaml:"defaultTemplateVersionId,omitempty"`
	DefaultVersion           string                `json:"defaultVersion,omitempty" yaml:"defaultVersion,omitempty"`
	Description              string                `json:"description,omitempty" yaml:"description,omitempty"`
	FolderName               string                `json:"folderName,omitempty" yaml:"folderName,omitempty"`
	Icon                     string                `json:"icon,omitempty" yaml:"icon,omitempty"`
	IconFilename             string                `json:"iconFilename,omitempty" yaml:"iconFilename,omitempty"`
	Labels                   map[string]string     `json:"labels,omitempty" yaml:"labels,omitempty"`
	Maintainer               string                `json:"maintainer,omitempty" yaml:"maintainer,omitempty"`
	Name                     string                `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences          []OwnerReference      `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Path                     string                `json:"path,omitempty" yaml:"path,omitempty"`
	ProjectCatalogID         string                `json:"projectCatalogID,omitempty" yaml:"projectCatalogID,omitempty"`
	ProjectID                string                `json:"projectID,omitempty" yaml:"projectID,omitempty"`
	ProjectURL               string                `json:"projectURL,omitempty" yaml:"projectURL,omitempty"`
	Readme                   string                `json:"readme,omitempty" yaml:"readme,omitempty"`
	Removed                  string                `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                    string                `json:"state,omitempty" yaml:"state,omitempty"`
	Status                   *TemplateStatus       `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning            string                `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage     string                `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                     string                `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UpgradeFrom              string                `json:"upgradeFrom,omitempty" yaml:"upgradeFrom,omitempty"`
	VersionLinks             map[string]string     `json:"versionLinks,omitempty" yaml:"versionLinks,omitempty"`
	Versions                 []TemplateVersionSpec `json:"versions,omitempty" yaml:"versions,omitempty"`
}

type TemplateCollection struct {
	types.Collection
	Data   []Template `json:"data,omitempty"`
	client *TemplateClient
}

type TemplateClient struct {
	apiClient *Client
}

type TemplateOperations interface {
	List(opts *types.ListOpts) (*TemplateCollection, error)
	Create(opts *Template) (*Template, error)
	Update(existing *Template, updates interface{}) (*Template, error)
	Replace(existing *Template) (*Template, error)
	ByID(id string) (*Template, error)
	Delete(container *Template) error
}

func newTemplateClient(apiClient *Client) *TemplateClient {
	return &TemplateClient{
		apiClient: apiClient,
	}
}

func (c *TemplateClient) Create(container *Template) (*Template, error) {
	resp := &Template{}
	err := c.apiClient.Ops.DoCreate(TemplateType, container, resp)
	return resp, err
}

func (c *TemplateClient) Update(existing *Template, updates interface{}) (*Template, error) {
	resp := &Template{}
	err := c.apiClient.Ops.DoUpdate(TemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *TemplateClient) Replace(obj *Template) (*Template, error) {
	resp := &Template{}
	err := c.apiClient.Ops.DoReplace(TemplateType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *TemplateClient) List(opts *types.ListOpts) (*TemplateCollection, error) {
	resp := &TemplateCollection{}
	err := c.apiClient.Ops.DoList(TemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *TemplateCollection) Next() (*TemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &TemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *TemplateClient) ByID(id string) (*Template, error) {
	resp := &Template{}
	err := c.apiClient.Ops.DoByID(TemplateType, id, resp)
	return resp, err
}

func (c *TemplateClient) Delete(container *Template) error {
	return c.apiClient.Ops.DoResourceDelete(TemplateType, &container.Resource)
}
