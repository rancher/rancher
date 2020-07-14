package client

import (
	"github.com/rancher/norman/types"
)

const (
	KontainerDriverType                      = "kontainerDriver"
	KontainerDriverFieldActive               = "active"
	KontainerDriverFieldActualURL            = "actualUrl"
	KontainerDriverFieldAnnotations          = "annotations"
	KontainerDriverFieldBuiltIn              = "builtIn"
	KontainerDriverFieldChecksum             = "checksum"
	KontainerDriverFieldConditions           = "conditions"
	KontainerDriverFieldCreated              = "created"
	KontainerDriverFieldCreatorID            = "creatorId"
	KontainerDriverFieldExecutablePath       = "executablePath"
	KontainerDriverFieldLabels               = "labels"
	KontainerDriverFieldName                 = "name"
	KontainerDriverFieldOwnerReferences      = "ownerReferences"
	KontainerDriverFieldRemoved              = "removed"
	KontainerDriverFieldState                = "state"
	KontainerDriverFieldTransitioning        = "transitioning"
	KontainerDriverFieldTransitioningMessage = "transitioningMessage"
	KontainerDriverFieldUIURL                = "uiUrl"
	KontainerDriverFieldURL                  = "url"
	KontainerDriverFieldUUID                 = "uuid"
	KontainerDriverFieldWhitelistDomains     = "whitelistDomains"
)

type KontainerDriver struct {
	types.Resource
	Active               bool              `json:"active,omitempty" yaml:"active,omitempty"`
	ActualURL            string            `json:"actualUrl,omitempty" yaml:"actualUrl,omitempty"`
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	BuiltIn              bool              `json:"builtIn,omitempty" yaml:"builtIn,omitempty"`
	Checksum             string            `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	Conditions           []Condition       `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ExecutablePath       string            `json:"executablePath,omitempty" yaml:"executablePath,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UIURL                string            `json:"uiUrl,omitempty" yaml:"uiUrl,omitempty"`
	URL                  string            `json:"url,omitempty" yaml:"url,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	WhitelistDomains     []string          `json:"whitelistDomains,omitempty" yaml:"whitelistDomains,omitempty"`
}

type KontainerDriverCollection struct {
	types.Collection
	Data   []KontainerDriver `json:"data,omitempty"`
	client *KontainerDriverClient
}

type KontainerDriverClient struct {
	apiClient *Client
}

type KontainerDriverOperations interface {
	List(opts *types.ListOpts) (*KontainerDriverCollection, error)
	ListAll(opts *types.ListOpts) (*KontainerDriverCollection, error)
	Create(opts *KontainerDriver) (*KontainerDriver, error)
	Update(existing *KontainerDriver, updates interface{}) (*KontainerDriver, error)
	Replace(existing *KontainerDriver) (*KontainerDriver, error)
	ByID(id string) (*KontainerDriver, error)
	Delete(container *KontainerDriver) error

	ActionActivate(resource *KontainerDriver) error

	ActionDeactivate(resource *KontainerDriver) error

	CollectionActionRefresh(resource *KontainerDriverCollection) error
}

func newKontainerDriverClient(apiClient *Client) *KontainerDriverClient {
	return &KontainerDriverClient{
		apiClient: apiClient,
	}
}

func (c *KontainerDriverClient) Create(container *KontainerDriver) (*KontainerDriver, error) {
	resp := &KontainerDriver{}
	err := c.apiClient.Ops.DoCreate(KontainerDriverType, container, resp)
	return resp, err
}

func (c *KontainerDriverClient) Update(existing *KontainerDriver, updates interface{}) (*KontainerDriver, error) {
	resp := &KontainerDriver{}
	err := c.apiClient.Ops.DoUpdate(KontainerDriverType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *KontainerDriverClient) Replace(obj *KontainerDriver) (*KontainerDriver, error) {
	resp := &KontainerDriver{}
	err := c.apiClient.Ops.DoReplace(KontainerDriverType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *KontainerDriverClient) List(opts *types.ListOpts) (*KontainerDriverCollection, error) {
	resp := &KontainerDriverCollection{}
	err := c.apiClient.Ops.DoList(KontainerDriverType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *KontainerDriverClient) ListAll(opts *types.ListOpts) (*KontainerDriverCollection, error) {
	resp := &KontainerDriverCollection{}
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

func (cc *KontainerDriverCollection) Next() (*KontainerDriverCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &KontainerDriverCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *KontainerDriverClient) ByID(id string) (*KontainerDriver, error) {
	resp := &KontainerDriver{}
	err := c.apiClient.Ops.DoByID(KontainerDriverType, id, resp)
	return resp, err
}

func (c *KontainerDriverClient) Delete(container *KontainerDriver) error {
	return c.apiClient.Ops.DoResourceDelete(KontainerDriverType, &container.Resource)
}

func (c *KontainerDriverClient) ActionActivate(resource *KontainerDriver) error {
	err := c.apiClient.Ops.DoAction(KontainerDriverType, "activate", &resource.Resource, nil, nil)
	return err
}

func (c *KontainerDriverClient) ActionDeactivate(resource *KontainerDriver) error {
	err := c.apiClient.Ops.DoAction(KontainerDriverType, "deactivate", &resource.Resource, nil, nil)
	return err
}

func (c *KontainerDriverClient) CollectionActionRefresh(resource *KontainerDriverCollection) error {
	err := c.apiClient.Ops.DoCollectionAction(KontainerDriverType, "refresh", &resource.Collection, nil, nil)
	return err
}
