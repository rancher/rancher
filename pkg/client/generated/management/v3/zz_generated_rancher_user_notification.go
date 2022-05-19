package client

import (
	"github.com/rancher/norman/types"
)

const (
	RancherUserNotificationType                 = "rancherUserNotification"
	RancherUserNotificationFieldAnnotations     = "annotations"
	RancherUserNotificationFieldComponentName   = "componentName"
	RancherUserNotificationFieldCreated         = "created"
	RancherUserNotificationFieldCreatorID       = "creatorId"
	RancherUserNotificationFieldLabels          = "labels"
	RancherUserNotificationFieldMessage         = "message"
	RancherUserNotificationFieldName            = "name"
	RancherUserNotificationFieldOwnerReferences = "ownerReferences"
	RancherUserNotificationFieldRemoved         = "removed"
	RancherUserNotificationFieldUUID            = "uuid"
)

type RancherUserNotification struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ComponentName   string            `json:"componentName,omitempty" yaml:"componentName,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Message         string            `json:"message,omitempty" yaml:"message,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type RancherUserNotificationCollection struct {
	types.Collection
	Data   []RancherUserNotification `json:"data,omitempty"`
	client *RancherUserNotificationClient
}

type RancherUserNotificationClient struct {
	apiClient *Client
}

type RancherUserNotificationOperations interface {
	List(opts *types.ListOpts) (*RancherUserNotificationCollection, error)
	ListAll(opts *types.ListOpts) (*RancherUserNotificationCollection, error)
	Create(opts *RancherUserNotification) (*RancherUserNotification, error)
	Update(existing *RancherUserNotification, updates interface{}) (*RancherUserNotification, error)
	Replace(existing *RancherUserNotification) (*RancherUserNotification, error)
	ByID(id string) (*RancherUserNotification, error)
	Delete(container *RancherUserNotification) error
}

func newRancherUserNotificationClient(apiClient *Client) *RancherUserNotificationClient {
	return &RancherUserNotificationClient{
		apiClient: apiClient,
	}
}

func (c *RancherUserNotificationClient) Create(container *RancherUserNotification) (*RancherUserNotification, error) {
	resp := &RancherUserNotification{}
	err := c.apiClient.Ops.DoCreate(RancherUserNotificationType, container, resp)
	return resp, err
}

func (c *RancherUserNotificationClient) Update(existing *RancherUserNotification, updates interface{}) (*RancherUserNotification, error) {
	resp := &RancherUserNotification{}
	err := c.apiClient.Ops.DoUpdate(RancherUserNotificationType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RancherUserNotificationClient) Replace(obj *RancherUserNotification) (*RancherUserNotification, error) {
	resp := &RancherUserNotification{}
	err := c.apiClient.Ops.DoReplace(RancherUserNotificationType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RancherUserNotificationClient) List(opts *types.ListOpts) (*RancherUserNotificationCollection, error) {
	resp := &RancherUserNotificationCollection{}
	err := c.apiClient.Ops.DoList(RancherUserNotificationType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *RancherUserNotificationClient) ListAll(opts *types.ListOpts) (*RancherUserNotificationCollection, error) {
	resp := &RancherUserNotificationCollection{}
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

func (cc *RancherUserNotificationCollection) Next() (*RancherUserNotificationCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RancherUserNotificationCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RancherUserNotificationClient) ByID(id string) (*RancherUserNotification, error) {
	resp := &RancherUserNotification{}
	err := c.apiClient.Ops.DoByID(RancherUserNotificationType, id, resp)
	return resp, err
}

func (c *RancherUserNotificationClient) Delete(container *RancherUserNotification) error {
	return c.apiClient.Ops.DoResourceDelete(RancherUserNotificationType, &container.Resource)
}
