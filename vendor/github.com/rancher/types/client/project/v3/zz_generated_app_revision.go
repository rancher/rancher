package client

import (
	"github.com/rancher/norman/types"
)

const (
	AppRevisionType                      = "appRevision"
	AppRevisionFieldAnnotations          = "annotations"
	AppRevisionFieldCreated              = "created"
	AppRevisionFieldCreatorID            = "creatorId"
	AppRevisionFieldLabels               = "labels"
	AppRevisionFieldName                 = "name"
	AppRevisionFieldNamespaceId          = "namespaceId"
	AppRevisionFieldOwnerReferences      = "ownerReferences"
	AppRevisionFieldRemoved              = "removed"
	AppRevisionFieldState                = "state"
	AppRevisionFieldStatus               = "status"
	AppRevisionFieldTransitioning        = "transitioning"
	AppRevisionFieldTransitioningMessage = "transitioningMessage"
	AppRevisionFieldUUID                 = "uuid"
)

type AppRevision struct {
	types.Resource
	Annotations          map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created              string             `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string             `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels               map[string]string  `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string             `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string             `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string             `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string             `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *AppRevisionStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type AppRevisionCollection struct {
	types.Collection
	Data   []AppRevision `json:"data,omitempty"`
	client *AppRevisionClient
}

type AppRevisionClient struct {
	apiClient *Client
}

type AppRevisionOperations interface {
	List(opts *types.ListOpts) (*AppRevisionCollection, error)
	Create(opts *AppRevision) (*AppRevision, error)
	Update(existing *AppRevision, updates interface{}) (*AppRevision, error)
	Replace(existing *AppRevision) (*AppRevision, error)
	ByID(id string) (*AppRevision, error)
	Delete(container *AppRevision) error
}

func newAppRevisionClient(apiClient *Client) *AppRevisionClient {
	return &AppRevisionClient{
		apiClient: apiClient,
	}
}

func (c *AppRevisionClient) Create(container *AppRevision) (*AppRevision, error) {
	resp := &AppRevision{}
	err := c.apiClient.Ops.DoCreate(AppRevisionType, container, resp)
	return resp, err
}

func (c *AppRevisionClient) Update(existing *AppRevision, updates interface{}) (*AppRevision, error) {
	resp := &AppRevision{}
	err := c.apiClient.Ops.DoUpdate(AppRevisionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *AppRevisionClient) Replace(obj *AppRevision) (*AppRevision, error) {
	resp := &AppRevision{}
	err := c.apiClient.Ops.DoReplace(AppRevisionType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *AppRevisionClient) List(opts *types.ListOpts) (*AppRevisionCollection, error) {
	resp := &AppRevisionCollection{}
	err := c.apiClient.Ops.DoList(AppRevisionType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *AppRevisionCollection) Next() (*AppRevisionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &AppRevisionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *AppRevisionClient) ByID(id string) (*AppRevision, error) {
	resp := &AppRevision{}
	err := c.apiClient.Ops.DoByID(AppRevisionType, id, resp)
	return resp, err
}

func (c *AppRevisionClient) Delete(container *AppRevision) error {
	return c.apiClient.Ops.DoResourceDelete(AppRevisionType, &container.Resource)
}
