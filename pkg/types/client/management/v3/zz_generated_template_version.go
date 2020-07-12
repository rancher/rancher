package client

import (
	"github.com/rancher/norman/types"
)

const (
	TemplateVersionType                      = "templateVersion"
	TemplateVersionFieldAnnotations          = "annotations"
	TemplateVersionFieldAppReadme            = "appReadme"
	TemplateVersionFieldCreated              = "created"
	TemplateVersionFieldCreatorID            = "creatorId"
	TemplateVersionFieldDigest               = "digest"
	TemplateVersionFieldExternalID           = "externalId"
	TemplateVersionFieldFiles                = "files"
	TemplateVersionFieldKubeVersion          = "kubeVersion"
	TemplateVersionFieldLabels               = "labels"
	TemplateVersionFieldName                 = "name"
	TemplateVersionFieldOwnerReferences      = "ownerReferences"
	TemplateVersionFieldQuestions            = "questions"
	TemplateVersionFieldRancherMaxVersion    = "rancherMaxVersion"
	TemplateVersionFieldRancherMinVersion    = "rancherMinVersion"
	TemplateVersionFieldRancherVersion       = "rancherVersion"
	TemplateVersionFieldReadme               = "readme"
	TemplateVersionFieldRemoved              = "removed"
	TemplateVersionFieldRequiredNamespace    = "requiredNamespace"
	TemplateVersionFieldState                = "state"
	TemplateVersionFieldStatus               = "status"
	TemplateVersionFieldTransitioning        = "transitioning"
	TemplateVersionFieldTransitioningMessage = "transitioningMessage"
	TemplateVersionFieldUUID                 = "uuid"
	TemplateVersionFieldUpgradeVersionLinks  = "upgradeVersionLinks"
	TemplateVersionFieldVersion              = "version"
	TemplateVersionFieldVersionDir           = "versionDir"
	TemplateVersionFieldVersionName          = "versionName"
	TemplateVersionFieldVersionURLs          = "versionUrls"
)

type TemplateVersion struct {
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

type TemplateVersionCollection struct {
	types.Collection
	Data   []TemplateVersion `json:"data,omitempty"`
	client *TemplateVersionClient
}

type TemplateVersionClient struct {
	apiClient *Client
}

type TemplateVersionOperations interface {
	List(opts *types.ListOpts) (*TemplateVersionCollection, error)
	ListAll(opts *types.ListOpts) (*TemplateVersionCollection, error)
	Create(opts *TemplateVersion) (*TemplateVersion, error)
	Update(existing *TemplateVersion, updates interface{}) (*TemplateVersion, error)
	Replace(existing *TemplateVersion) (*TemplateVersion, error)
	ByID(id string) (*TemplateVersion, error)
	Delete(container *TemplateVersion) error
}

func newTemplateVersionClient(apiClient *Client) *TemplateVersionClient {
	return &TemplateVersionClient{
		apiClient: apiClient,
	}
}

func (c *TemplateVersionClient) Create(container *TemplateVersion) (*TemplateVersion, error) {
	resp := &TemplateVersion{}
	err := c.apiClient.Ops.DoCreate(TemplateVersionType, container, resp)
	return resp, err
}

func (c *TemplateVersionClient) Update(existing *TemplateVersion, updates interface{}) (*TemplateVersion, error) {
	resp := &TemplateVersion{}
	err := c.apiClient.Ops.DoUpdate(TemplateVersionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *TemplateVersionClient) Replace(obj *TemplateVersion) (*TemplateVersion, error) {
	resp := &TemplateVersion{}
	err := c.apiClient.Ops.DoReplace(TemplateVersionType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *TemplateVersionClient) List(opts *types.ListOpts) (*TemplateVersionCollection, error) {
	resp := &TemplateVersionCollection{}
	err := c.apiClient.Ops.DoList(TemplateVersionType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *TemplateVersionClient) ListAll(opts *types.ListOpts) (*TemplateVersionCollection, error) {
	resp := &TemplateVersionCollection{}
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

func (cc *TemplateVersionCollection) Next() (*TemplateVersionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &TemplateVersionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *TemplateVersionClient) ByID(id string) (*TemplateVersion, error) {
	resp := &TemplateVersion{}
	err := c.apiClient.Ops.DoByID(TemplateVersionType, id, resp)
	return resp, err
}

func (c *TemplateVersionClient) Delete(container *TemplateVersion) error {
	return c.apiClient.Ops.DoResourceDelete(TemplateVersionType, &container.Resource)
}
