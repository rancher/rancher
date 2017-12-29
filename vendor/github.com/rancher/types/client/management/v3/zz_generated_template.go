package client

import (
	"github.com/rancher/norman/types"
)

const (
	TemplateType                          = "template"
	TemplateFieldAnnotations              = "annotations"
	TemplateFieldBase                     = "templateBase"
	TemplateFieldCatalogID                = "catalogId"
	TemplateFieldCategories               = "categories"
	TemplateFieldCategory                 = "category"
	TemplateFieldCreated                  = "created"
	TemplateFieldCreatorID                = "creatorId"
	TemplateFieldDefaultTemplateVersionID = "defaultTemplateVersionId"
	TemplateFieldDefaultVersion           = "defaultVersion"
	TemplateFieldDescription              = "description"
	TemplateFieldFinalizers               = "finalizers"
	TemplateFieldFolderName               = "folderName"
	TemplateFieldIcon                     = "icon"
	TemplateFieldIconFilename             = "iconFilename"
	TemplateFieldIsSystem                 = "isSystem"
	TemplateFieldLabels                   = "labels"
	TemplateFieldLicense                  = "license"
	TemplateFieldMaintainer               = "maintainer"
	TemplateFieldName                     = "name"
	TemplateFieldOwnerReferences          = "ownerReferences"
	TemplateFieldPath                     = "path"
	TemplateFieldProjectURL               = "projectURL"
	TemplateFieldReadme                   = "readme"
	TemplateFieldRemoved                  = "removed"
	TemplateFieldState                    = "state"
	TemplateFieldStatus                   = "status"
	TemplateFieldTransitioning            = "transitioning"
	TemplateFieldTransitioningMessage     = "transitioningMessage"
	TemplateFieldUpgradeFrom              = "upgradeFrom"
	TemplateFieldUuid                     = "uuid"
	TemplateFieldVersions                 = "versions"
)

type Template struct {
	types.Resource
	Annotations              map[string]string     `json:"annotations,omitempty"`
	Base                     string                `json:"templateBase,omitempty"`
	CatalogID                string                `json:"catalogId,omitempty"`
	Categories               []string              `json:"categories,omitempty"`
	Category                 string                `json:"category,omitempty"`
	Created                  string                `json:"created,omitempty"`
	CreatorID                string                `json:"creatorId,omitempty"`
	DefaultTemplateVersionID string                `json:"defaultTemplateVersionId,omitempty"`
	DefaultVersion           string                `json:"defaultVersion,omitempty"`
	Description              string                `json:"description,omitempty"`
	Finalizers               []string              `json:"finalizers,omitempty"`
	FolderName               string                `json:"folderName,omitempty"`
	Icon                     string                `json:"icon,omitempty"`
	IconFilename             string                `json:"iconFilename,omitempty"`
	IsSystem                 string                `json:"isSystem,omitempty"`
	Labels                   map[string]string     `json:"labels,omitempty"`
	License                  string                `json:"license,omitempty"`
	Maintainer               string                `json:"maintainer,omitempty"`
	Name                     string                `json:"name,omitempty"`
	OwnerReferences          []OwnerReference      `json:"ownerReferences,omitempty"`
	Path                     string                `json:"path,omitempty"`
	ProjectURL               string                `json:"projectURL,omitempty"`
	Readme                   string                `json:"readme,omitempty"`
	Removed                  string                `json:"removed,omitempty"`
	State                    string                `json:"state,omitempty"`
	Status                   *TemplateStatus       `json:"status,omitempty"`
	Transitioning            string                `json:"transitioning,omitempty"`
	TransitioningMessage     string                `json:"transitioningMessage,omitempty"`
	UpgradeFrom              string                `json:"upgradeFrom,omitempty"`
	Uuid                     string                `json:"uuid,omitempty"`
	Versions                 []TemplateVersionSpec `json:"versions,omitempty"`
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
