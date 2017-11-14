package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterEventType                 = "clusterEvent"
	ClusterEventFieldAnnotations     = "annotations"
	ClusterEventFieldClusterName     = "clusterName"
	ClusterEventFieldCount           = "count"
	ClusterEventFieldCreated         = "created"
	ClusterEventFieldFinalizers      = "finalizers"
	ClusterEventFieldFirstTimestamp  = "firstTimestamp"
	ClusterEventFieldInvolvedObject  = "involvedObject"
	ClusterEventFieldLabels          = "labels"
	ClusterEventFieldLastTimestamp   = "lastTimestamp"
	ClusterEventFieldMessage         = "message"
	ClusterEventFieldName            = "name"
	ClusterEventFieldOwnerReferences = "ownerReferences"
	ClusterEventFieldReason          = "reason"
	ClusterEventFieldRemoved         = "removed"
	ClusterEventFieldResourcePath    = "resourcePath"
	ClusterEventFieldSource          = "source"
	ClusterEventFieldType            = "type"
	ClusterEventFieldUuid            = "uuid"
)

type ClusterEvent struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	ClusterName     string            `json:"clusterName,omitempty"`
	Count           *int64            `json:"count,omitempty"`
	Created         string            `json:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	FirstTimestamp  string            `json:"firstTimestamp,omitempty"`
	InvolvedObject  *ObjectReference  `json:"involvedObject,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	LastTimestamp   string            `json:"lastTimestamp,omitempty"`
	Message         string            `json:"message,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Reason          string            `json:"reason,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	ResourcePath    string            `json:"resourcePath,omitempty"`
	Source          *EventSource      `json:"source,omitempty"`
	Type            string            `json:"type,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
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
