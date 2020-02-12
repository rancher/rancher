package client

import (
	"github.com/rancher/norman/types"
)

const (
	ServiceMonitorType                   = "serviceMonitor"
	ServiceMonitorFieldAnnotations       = "annotations"
	ServiceMonitorFieldCreated           = "created"
	ServiceMonitorFieldCreatorID         = "creatorId"
	ServiceMonitorFieldEndpoints         = "endpoints"
	ServiceMonitorFieldJobLabel          = "jobLabel"
	ServiceMonitorFieldLabels            = "labels"
	ServiceMonitorFieldName              = "name"
	ServiceMonitorFieldNamespaceId       = "namespaceId"
	ServiceMonitorFieldNamespaceSelector = "namespaceSelector"
	ServiceMonitorFieldOwnerReferences   = "ownerReferences"
	ServiceMonitorFieldPodTargetLabels   = "podTargetLabels"
	ServiceMonitorFieldProjectID         = "projectId"
	ServiceMonitorFieldRemoved           = "removed"
	ServiceMonitorFieldSampleLimit       = "sampleLimit"
	ServiceMonitorFieldSelector          = "selector"
	ServiceMonitorFieldTargetLabels      = "targetLabels"
	ServiceMonitorFieldTargetService     = "targetService"
	ServiceMonitorFieldTargetWorkload    = "targetWorkload"
	ServiceMonitorFieldUUID              = "uuid"
)

type ServiceMonitor struct {
	types.Resource
	Annotations       map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created           string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID         string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Endpoints         []Endpoint        `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	JobLabel          string            `json:"jobLabel,omitempty" yaml:"jobLabel,omitempty"`
	Labels            map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name              string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId       string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NamespaceSelector []string          `json:"namespaceSelector,omitempty" yaml:"namespaceSelector,omitempty"`
	OwnerReferences   []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PodTargetLabels   []string          `json:"podTargetLabels,omitempty" yaml:"podTargetLabels,omitempty"`
	ProjectID         string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed           string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SampleLimit       int64             `json:"sampleLimit,omitempty" yaml:"sampleLimit,omitempty"`
	Selector          *LabelSelector    `json:"selector,omitempty" yaml:"selector,omitempty"`
	TargetLabels      []string          `json:"targetLabels,omitempty" yaml:"targetLabels,omitempty"`
	TargetService     string            `json:"targetService,omitempty" yaml:"targetService,omitempty"`
	TargetWorkload    string            `json:"targetWorkload,omitempty" yaml:"targetWorkload,omitempty"`
	UUID              string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ServiceMonitorCollection struct {
	types.Collection
	Data   []ServiceMonitor `json:"data,omitempty"`
	client *ServiceMonitorClient
}

type ServiceMonitorClient struct {
	apiClient *Client
}

type ServiceMonitorOperations interface {
	List(opts *types.ListOpts) (*ServiceMonitorCollection, error)
	ListAll(opts *types.ListOpts) (*ServiceMonitorCollection, error)
	Create(opts *ServiceMonitor) (*ServiceMonitor, error)
	Update(existing *ServiceMonitor, updates interface{}) (*ServiceMonitor, error)
	Replace(existing *ServiceMonitor) (*ServiceMonitor, error)
	ByID(id string) (*ServiceMonitor, error)
	Delete(container *ServiceMonitor) error
}

func newServiceMonitorClient(apiClient *Client) *ServiceMonitorClient {
	return &ServiceMonitorClient{
		apiClient: apiClient,
	}
}

func (c *ServiceMonitorClient) Create(container *ServiceMonitor) (*ServiceMonitor, error) {
	resp := &ServiceMonitor{}
	err := c.apiClient.Ops.DoCreate(ServiceMonitorType, container, resp)
	return resp, err
}

func (c *ServiceMonitorClient) Update(existing *ServiceMonitor, updates interface{}) (*ServiceMonitor, error) {
	resp := &ServiceMonitor{}
	err := c.apiClient.Ops.DoUpdate(ServiceMonitorType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ServiceMonitorClient) Replace(obj *ServiceMonitor) (*ServiceMonitor, error) {
	resp := &ServiceMonitor{}
	err := c.apiClient.Ops.DoReplace(ServiceMonitorType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ServiceMonitorClient) List(opts *types.ListOpts) (*ServiceMonitorCollection, error) {
	resp := &ServiceMonitorCollection{}
	err := c.apiClient.Ops.DoList(ServiceMonitorType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ServiceMonitorClient) ListAll(opts *types.ListOpts) (*ServiceMonitorCollection, error) {
	resp := &ServiceMonitorCollection{}
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

func (cc *ServiceMonitorCollection) Next() (*ServiceMonitorCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ServiceMonitorCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ServiceMonitorClient) ByID(id string) (*ServiceMonitor, error) {
	resp := &ServiceMonitor{}
	err := c.apiClient.Ops.DoByID(ServiceMonitorType, id, resp)
	return resp, err
}

func (c *ServiceMonitorClient) Delete(container *ServiceMonitor) error {
	return c.apiClient.Ops.DoResourceDelete(ServiceMonitorType, &container.Resource)
}
