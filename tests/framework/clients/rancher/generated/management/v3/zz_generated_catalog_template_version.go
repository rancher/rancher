package client

import (
	"github.com/rancher/norman/types"
)

const (
	CatalogTemplateVersionType                      = "catalogTemplateVersion"
	CatalogTemplateVersionFieldAnnotations          = "annotations"
	CatalogTemplateVersionFieldAppReadme            = "appReadme"
	CatalogTemplateVersionFieldCreated              = "created"
	CatalogTemplateVersionFieldCreatorID            = "creatorId"
	CatalogTemplateVersionFieldDigest               = "digest"
	CatalogTemplateVersionFieldExternalID           = "externalId"
	CatalogTemplateVersionFieldFiles                = "files"
	CatalogTemplateVersionFieldKubeVersion          = "kubeVersion"
	CatalogTemplateVersionFieldLabels               = "labels"
	CatalogTemplateVersionFieldName                 = "name"
	CatalogTemplateVersionFieldOwnerReferences      = "ownerReferences"
	CatalogTemplateVersionFieldQuestions            = "questions"
	CatalogTemplateVersionFieldRancherMaxVersion    = "rancherMaxVersion"
	CatalogTemplateVersionFieldRancherMinVersion    = "rancherMinVersion"
	CatalogTemplateVersionFieldRancherVersion       = "rancherVersion"
	CatalogTemplateVersionFieldReadme               = "readme"
	CatalogTemplateVersionFieldRemoved              = "removed"
	CatalogTemplateVersionFieldRequiredNamespace    = "requiredNamespace"
	CatalogTemplateVersionFieldState                = "state"
	CatalogTemplateVersionFieldStatus               = "status"
	CatalogTemplateVersionFieldTransitioning        = "transitioning"
	CatalogTemplateVersionFieldTransitioningMessage = "transitioningMessage"
	CatalogTemplateVersionFieldUUID                 = "uuid"
	CatalogTemplateVersionFieldUpgradeVersionLinks  = "upgradeVersionLinks"
	CatalogTemplateVersionFieldVersion              = "version"
	CatalogTemplateVersionFieldVersionDir           = "versionDir"
	CatalogTemplateVersionFieldVersionName          = "versionName"
	CatalogTemplateVersionFieldVersionURLs          = "versionUrls"
)

type CatalogTemplateVersion struct {
	types.Resource
	Annotations          map[string]string      `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AppReadme            string                 `json:"appReadme,omitempty" yaml:"appReadme,omitempty"`
	Created              string                 `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string                 `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Digest               string                 `json:"digest,omitempty" yaml:"digest,omitempty"`
	ExternalID           string                 `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files                map[string]string      `json:"files,omitempty" yaml:"files,omitempty"`
	KubeVersion          string                 `json:"kubeVersion,omitempty" yaml:"kubeVersion,omitempty"`
	Labels               map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string                 `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference       `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Questions            []Question             `json:"questions,omitempty" yaml:"questions,omitempty"`
	RancherMaxVersion    string                 `json:"rancherMaxVersion,omitempty" yaml:"rancherMaxVersion,omitempty"`
	RancherMinVersion    string                 `json:"rancherMinVersion,omitempty" yaml:"rancherMinVersion,omitempty"`
	RancherVersion       string                 `json:"rancherVersion,omitempty" yaml:"rancherVersion,omitempty"`
	Readme               string                 `json:"readme,omitempty" yaml:"readme,omitempty"`
	Removed              string                 `json:"removed,omitempty" yaml:"removed,omitempty"`
	RequiredNamespace    string                 `json:"requiredNamespace,omitempty" yaml:"requiredNamespace,omitempty"`
	State                string                 `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *TemplateVersionStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string                 `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string                 `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string                 `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UpgradeVersionLinks  map[string]string      `json:"upgradeVersionLinks,omitempty" yaml:"upgradeVersionLinks,omitempty"`
	Version              string                 `json:"version,omitempty" yaml:"version,omitempty"`
	VersionDir           string                 `json:"versionDir,omitempty" yaml:"versionDir,omitempty"`
	VersionName          string                 `json:"versionName,omitempty" yaml:"versionName,omitempty"`
	VersionURLs          []string               `json:"versionUrls,omitempty" yaml:"versionUrls,omitempty"`
}

type CatalogTemplateVersionCollection struct {
	types.Collection
	Data   []CatalogTemplateVersion `json:"data,omitempty"`
	client *CatalogTemplateVersionClient
}

type CatalogTemplateVersionClient struct {
	apiClient *Client
}

type CatalogTemplateVersionOperations interface {
	List(opts *types.ListOpts) (*CatalogTemplateVersionCollection, error)
	ListAll(opts *types.ListOpts) (*CatalogTemplateVersionCollection, error)
	Create(opts *CatalogTemplateVersion) (*CatalogTemplateVersion, error)
	Update(existing *CatalogTemplateVersion, updates interface{}) (*CatalogTemplateVersion, error)
	Replace(existing *CatalogTemplateVersion) (*CatalogTemplateVersion, error)
	ByID(id string) (*CatalogTemplateVersion, error)
	Delete(container *CatalogTemplateVersion) error
}

func newCatalogTemplateVersionClient(apiClient *Client) *CatalogTemplateVersionClient {
	return &CatalogTemplateVersionClient{
		apiClient: apiClient,
	}
}

func (c *CatalogTemplateVersionClient) Create(container *CatalogTemplateVersion) (*CatalogTemplateVersion, error) {
	resp := &CatalogTemplateVersion{}
	err := c.apiClient.Ops.DoCreate(CatalogTemplateVersionType, container, resp)
	return resp, err
}

func (c *CatalogTemplateVersionClient) Update(existing *CatalogTemplateVersion, updates interface{}) (*CatalogTemplateVersion, error) {
	resp := &CatalogTemplateVersion{}
	err := c.apiClient.Ops.DoUpdate(CatalogTemplateVersionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CatalogTemplateVersionClient) Replace(obj *CatalogTemplateVersion) (*CatalogTemplateVersion, error) {
	resp := &CatalogTemplateVersion{}
	err := c.apiClient.Ops.DoReplace(CatalogTemplateVersionType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *CatalogTemplateVersionClient) List(opts *types.ListOpts) (*CatalogTemplateVersionCollection, error) {
	resp := &CatalogTemplateVersionCollection{}
	err := c.apiClient.Ops.DoList(CatalogTemplateVersionType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *CatalogTemplateVersionClient) ListAll(opts *types.ListOpts) (*CatalogTemplateVersionCollection, error) {
	resp := &CatalogTemplateVersionCollection{}
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

func (cc *CatalogTemplateVersionCollection) Next() (*CatalogTemplateVersionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CatalogTemplateVersionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CatalogTemplateVersionClient) ByID(id string) (*CatalogTemplateVersion, error) {
	resp := &CatalogTemplateVersion{}
	err := c.apiClient.Ops.DoByID(CatalogTemplateVersionType, id, resp)
	return resp, err
}

func (c *CatalogTemplateVersionClient) Delete(container *CatalogTemplateVersion) error {
	return c.apiClient.Ops.DoResourceDelete(CatalogTemplateVersionType, &container.Resource)
}
