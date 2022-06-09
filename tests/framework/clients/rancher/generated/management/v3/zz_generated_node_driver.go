package client

import (
	"github.com/rancher/norman/types"
)

const (
	NodeDriverType                      = "nodeDriver"
	NodeDriverFieldActive               = "active"
	NodeDriverFieldAddCloudCredential   = "addCloudCredential"
	NodeDriverFieldAnnotations          = "annotations"
	NodeDriverFieldBuiltin              = "builtin"
	NodeDriverFieldChecksum             = "checksum"
	NodeDriverFieldCreated              = "created"
	NodeDriverFieldCreatorID            = "creatorId"
	NodeDriverFieldDescription          = "description"
	NodeDriverFieldExternalID           = "externalId"
	NodeDriverFieldLabels               = "labels"
	NodeDriverFieldName                 = "name"
	NodeDriverFieldOwnerReferences      = "ownerReferences"
	NodeDriverFieldRemoved              = "removed"
	NodeDriverFieldState                = "state"
	NodeDriverFieldStatus               = "status"
	NodeDriverFieldTransitioning        = "transitioning"
	NodeDriverFieldTransitioningMessage = "transitioningMessage"
	NodeDriverFieldUIURL                = "uiUrl"
	NodeDriverFieldURL                  = "url"
	NodeDriverFieldUUID                 = "uuid"
	NodeDriverFieldWhitelistDomains     = "whitelistDomains"
)

type NodeDriver struct {
	types.Resource
	Active               bool              `json:"active,omitempty" yaml:"active,omitempty"`
	AddCloudCredential   bool              `json:"addCloudCredential,omitempty" yaml:"addCloudCredential,omitempty"`
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Builtin              bool              `json:"builtin,omitempty" yaml:"builtin,omitempty"`
	Checksum             string            `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description          string            `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalID           string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *NodeDriverStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UIURL                string            `json:"uiUrl,omitempty" yaml:"uiUrl,omitempty"`
	URL                  string            `json:"url,omitempty" yaml:"url,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	WhitelistDomains     []string          `json:"whitelistDomains,omitempty" yaml:"whitelistDomains,omitempty"`
}

type NodeDriverCollection struct {
	types.Collection
	Data   []NodeDriver `json:"data,omitempty"`
	client *NodeDriverClient
}

type NodeDriverClient struct {
	apiClient *Client
}

type NodeDriverOperations interface {
	List(opts *types.ListOpts) (*NodeDriverCollection, error)
	ListAll(opts *types.ListOpts) (*NodeDriverCollection, error)
	Create(opts *NodeDriver) (*NodeDriver, error)
	Update(existing *NodeDriver, updates interface{}) (*NodeDriver, error)
	Replace(existing *NodeDriver) (*NodeDriver, error)
	ByID(id string) (*NodeDriver, error)
	Delete(container *NodeDriver) error

	ActionActivate(resource *NodeDriver) (*NodeDriver, error)

	ActionDeactivate(resource *NodeDriver) (*NodeDriver, error)
}

func newNodeDriverClient(apiClient *Client) *NodeDriverClient {
	return &NodeDriverClient{
		apiClient: apiClient,
	}
}

func (c *NodeDriverClient) Create(container *NodeDriver) (*NodeDriver, error) {
	resp := &NodeDriver{}
	err := c.apiClient.Ops.DoCreate(NodeDriverType, container, resp)
	return resp, err
}

func (c *NodeDriverClient) Update(existing *NodeDriver, updates interface{}) (*NodeDriver, error) {
	resp := &NodeDriver{}
	err := c.apiClient.Ops.DoUpdate(NodeDriverType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NodeDriverClient) Replace(obj *NodeDriver) (*NodeDriver, error) {
	resp := &NodeDriver{}
	err := c.apiClient.Ops.DoReplace(NodeDriverType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *NodeDriverClient) List(opts *types.ListOpts) (*NodeDriverCollection, error) {
	resp := &NodeDriverCollection{}
	err := c.apiClient.Ops.DoList(NodeDriverType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *NodeDriverClient) ListAll(opts *types.ListOpts) (*NodeDriverCollection, error) {
	resp := &NodeDriverCollection{}
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

func (cc *NodeDriverCollection) Next() (*NodeDriverCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NodeDriverCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NodeDriverClient) ByID(id string) (*NodeDriver, error) {
	resp := &NodeDriver{}
	err := c.apiClient.Ops.DoByID(NodeDriverType, id, resp)
	return resp, err
}

func (c *NodeDriverClient) Delete(container *NodeDriver) error {
	return c.apiClient.Ops.DoResourceDelete(NodeDriverType, &container.Resource)
}

func (c *NodeDriverClient) ActionActivate(resource *NodeDriver) (*NodeDriver, error) {
	resp := &NodeDriver{}
	err := c.apiClient.Ops.DoAction(NodeDriverType, "activate", &resource.Resource, nil, resp)
	return resp, err
}

func (c *NodeDriverClient) ActionDeactivate(resource *NodeDriver) (*NodeDriver, error) {
	resp := &NodeDriver{}
	err := c.apiClient.Ops.DoAction(NodeDriverType, "deactivate", &resource.Resource, nil, resp)
	return resp, err
}
