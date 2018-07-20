package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterAlertType                       = "clusterAlert"
	ClusterAlertFieldAlertState            = "alertState"
	ClusterAlertFieldAnnotations           = "annotations"
	ClusterAlertFieldClusterID             = "clusterId"
	ClusterAlertFieldCreated               = "created"
	ClusterAlertFieldCreatorID             = "creatorId"
	ClusterAlertFieldDescription           = "description"
	ClusterAlertFieldInitialWaitSeconds    = "initialWaitSeconds"
	ClusterAlertFieldLabels                = "labels"
	ClusterAlertFieldName                  = "name"
	ClusterAlertFieldNamespaceId           = "namespaceId"
	ClusterAlertFieldOwnerReferences       = "ownerReferences"
	ClusterAlertFieldRecipients            = "recipients"
	ClusterAlertFieldRemoved               = "removed"
	ClusterAlertFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ClusterAlertFieldSeverity              = "severity"
	ClusterAlertFieldState                 = "state"
	ClusterAlertFieldTargetEvent           = "targetEvent"
	ClusterAlertFieldTargetNode            = "targetNode"
	ClusterAlertFieldTargetSystemService   = "targetSystemService"
	ClusterAlertFieldTransitioning         = "transitioning"
	ClusterAlertFieldTransitioningMessage  = "transitioningMessage"
	ClusterAlertFieldUUID                  = "uuid"
)

type ClusterAlert struct {
	types.Resource
	AlertState            string               `json:"alertState,omitempty" yaml:"alertState,omitempty"`
	Annotations           map[string]string    `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterID             string               `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created               string               `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID             string               `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description           string               `json:"description,omitempty" yaml:"description,omitempty"`
	InitialWaitSeconds    int64                `json:"initialWaitSeconds,omitempty" yaml:"initialWaitSeconds,omitempty"`
	Labels                map[string]string    `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                  string               `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId           string               `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences       []OwnerReference     `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Recipients            []Recipient          `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	Removed               string               `json:"removed,omitempty" yaml:"removed,omitempty"`
	RepeatIntervalSeconds int64                `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	Severity              string               `json:"severity,omitempty" yaml:"severity,omitempty"`
	State                 string               `json:"state,omitempty" yaml:"state,omitempty"`
	TargetEvent           *TargetEvent         `json:"targetEvent,omitempty" yaml:"targetEvent,omitempty"`
	TargetNode            *TargetNode          `json:"targetNode,omitempty" yaml:"targetNode,omitempty"`
	TargetSystemService   *TargetSystemService `json:"targetSystemService,omitempty" yaml:"targetSystemService,omitempty"`
	Transitioning         string               `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage  string               `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                  string               `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ClusterAlertCollection struct {
	types.Collection
	Data   []ClusterAlert `json:"data,omitempty"`
	client *ClusterAlertClient
}

type ClusterAlertClient struct {
	apiClient *Client
}

type ClusterAlertOperations interface {
	List(opts *types.ListOpts) (*ClusterAlertCollection, error)
	Create(opts *ClusterAlert) (*ClusterAlert, error)
	Update(existing *ClusterAlert, updates interface{}) (*ClusterAlert, error)
	Replace(existing *ClusterAlert) (*ClusterAlert, error)
	ByID(id string) (*ClusterAlert, error)
	Delete(container *ClusterAlert) error

	ActionActivate(resource *ClusterAlert) error

	ActionDeactivate(resource *ClusterAlert) error

	ActionMute(resource *ClusterAlert) error

	ActionUnmute(resource *ClusterAlert) error
}

func newClusterAlertClient(apiClient *Client) *ClusterAlertClient {
	return &ClusterAlertClient{
		apiClient: apiClient,
	}
}

func (c *ClusterAlertClient) Create(container *ClusterAlert) (*ClusterAlert, error) {
	resp := &ClusterAlert{}
	err := c.apiClient.Ops.DoCreate(ClusterAlertType, container, resp)
	return resp, err
}

func (c *ClusterAlertClient) Update(existing *ClusterAlert, updates interface{}) (*ClusterAlert, error) {
	resp := &ClusterAlert{}
	err := c.apiClient.Ops.DoUpdate(ClusterAlertType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterAlertClient) Replace(obj *ClusterAlert) (*ClusterAlert, error) {
	resp := &ClusterAlert{}
	err := c.apiClient.Ops.DoReplace(ClusterAlertType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterAlertClient) List(opts *types.ListOpts) (*ClusterAlertCollection, error) {
	resp := &ClusterAlertCollection{}
	err := c.apiClient.Ops.DoList(ClusterAlertType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterAlertCollection) Next() (*ClusterAlertCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterAlertCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterAlertClient) ByID(id string) (*ClusterAlert, error) {
	resp := &ClusterAlert{}
	err := c.apiClient.Ops.DoByID(ClusterAlertType, id, resp)
	return resp, err
}

func (c *ClusterAlertClient) Delete(container *ClusterAlert) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterAlertType, &container.Resource)
}

func (c *ClusterAlertClient) ActionActivate(resource *ClusterAlert) error {
	err := c.apiClient.Ops.DoAction(ClusterAlertType, "activate", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterAlertClient) ActionDeactivate(resource *ClusterAlert) error {
	err := c.apiClient.Ops.DoAction(ClusterAlertType, "deactivate", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterAlertClient) ActionMute(resource *ClusterAlert) error {
	err := c.apiClient.Ops.DoAction(ClusterAlertType, "mute", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterAlertClient) ActionUnmute(resource *ClusterAlert) error {
	err := c.apiClient.Ops.DoAction(ClusterAlertType, "unmute", &resource.Resource, nil, nil)
	return err
}
