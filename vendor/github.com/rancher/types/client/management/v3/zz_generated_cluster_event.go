package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterEventType                 = "clusterEvent"
	ClusterEventFieldAnnotations     = "annotations"
	ClusterEventFieldClusterId       = "clusterId"
	ClusterEventFieldCount           = "count"
	ClusterEventFieldCreated         = "created"
	ClusterEventFieldCreatorID       = "creatorId"
	ClusterEventFieldEventType       = "eventType"
	ClusterEventFieldFirstTimestamp  = "firstTimestamp"
	ClusterEventFieldInvolvedObject  = "involvedObject"
	ClusterEventFieldLabels          = "labels"
	ClusterEventFieldLastTimestamp   = "lastTimestamp"
	ClusterEventFieldMessage         = "message"
	ClusterEventFieldName            = "name"
	ClusterEventFieldNamespaceId     = "namespaceId"
	ClusterEventFieldOwnerReferences = "ownerReferences"
	ClusterEventFieldReason          = "reason"
	ClusterEventFieldRemoved         = "removed"
	ClusterEventFieldSource          = "source"
	ClusterEventFieldUuid            = "uuid"
)

type ClusterEvent struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterId       string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Count           *int64            `json:"count,omitempty" yaml:"count,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	EventType       string            `json:"eventType,omitempty" yaml:"eventType,omitempty"`
	FirstTimestamp  string            `json:"firstTimestamp,omitempty" yaml:"firstTimestamp,omitempty"`
	InvolvedObject  *ObjectReference  `json:"involvedObject,omitempty" yaml:"involvedObject,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LastTimestamp   string            `json:"lastTimestamp,omitempty" yaml:"lastTimestamp,omitempty"`
	Message         string            `json:"message,omitempty" yaml:"message,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Reason          string            `json:"reason,omitempty" yaml:"reason,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Source          *EventSource      `json:"source,omitempty" yaml:"source,omitempty"`
	Uuid            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type ClusterEventCollection struct {
	types.Collection
	Data   []ClusterEvent `json:"data,omitempty"`
	client *ClusterEventClient
}

type ClusterEventClient struct {
	apiClient *Client
}

type ClusterEventOperations interface {
	List(opts *types.ListOpts) (*ClusterEventCollection, error)
	Create(opts *ClusterEvent) (*ClusterEvent, error)
	Update(existing *ClusterEvent, updates interface{}) (*ClusterEvent, error)
	ByID(id string) (*ClusterEvent, error)
	Delete(container *ClusterEvent) error
}

func newClusterEventClient(apiClient *Client) *ClusterEventClient {
	return &ClusterEventClient{
		apiClient: apiClient,
	}
}

func (c *ClusterEventClient) Create(container *ClusterEvent) (*ClusterEvent, error) {
	resp := &ClusterEvent{}
	err := c.apiClient.Ops.DoCreate(ClusterEventType, container, resp)
	return resp, err
}

func (c *ClusterEventClient) Update(existing *ClusterEvent, updates interface{}) (*ClusterEvent, error) {
	resp := &ClusterEvent{}
	err := c.apiClient.Ops.DoUpdate(ClusterEventType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterEventClient) List(opts *types.ListOpts) (*ClusterEventCollection, error) {
	resp := &ClusterEventCollection{}
	err := c.apiClient.Ops.DoList(ClusterEventType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterEventCollection) Next() (*ClusterEventCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterEventCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterEventClient) ByID(id string) (*ClusterEvent, error) {
	resp := &ClusterEvent{}
	err := c.apiClient.Ops.DoByID(ClusterEventType, id, resp)
	return resp, err
}

func (c *ClusterEventClient) Delete(container *ClusterEvent) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterEventType, &container.Resource)
}
