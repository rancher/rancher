package client

import (
	"github.com/rancher/norman/types"
)

const (
	ProjectMonitorGraphType                        = "projectMonitorGraph"
	ProjectMonitorGraphFieldAnnotations            = "annotations"
	ProjectMonitorGraphFieldCreated                = "created"
	ProjectMonitorGraphFieldCreatorID              = "creatorId"
	ProjectMonitorGraphFieldDescription            = "description"
	ProjectMonitorGraphFieldDetailsMetricsSelector = "detailsMetricsSelector"
	ProjectMonitorGraphFieldDisplayResourceType    = "displayResourceType"
	ProjectMonitorGraphFieldGraphType              = "graphType"
	ProjectMonitorGraphFieldLabels                 = "labels"
	ProjectMonitorGraphFieldMetricsSelector        = "metricsSelector"
	ProjectMonitorGraphFieldName                   = "name"
	ProjectMonitorGraphFieldNamespaceId            = "namespaceId"
	ProjectMonitorGraphFieldOwnerReferences        = "ownerReferences"
	ProjectMonitorGraphFieldPriority               = "priority"
	ProjectMonitorGraphFieldProjectID              = "projectId"
	ProjectMonitorGraphFieldRemoved                = "removed"
	ProjectMonitorGraphFieldResourceType           = "resourceType"
	ProjectMonitorGraphFieldUUID                   = "uuid"
	ProjectMonitorGraphFieldYAxis                  = "yAxis"
)

type ProjectMonitorGraph struct {
	types.Resource
	Annotations            map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created                string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID              string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description            string            `json:"description,omitempty" yaml:"description,omitempty"`
	DetailsMetricsSelector map[string]string `json:"detailsMetricsSelector,omitempty" yaml:"detailsMetricsSelector,omitempty"`
	DisplayResourceType    string            `json:"displayResourceType,omitempty" yaml:"displayResourceType,omitempty"`
	GraphType              string            `json:"graphType,omitempty" yaml:"graphType,omitempty"`
	Labels                 map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	MetricsSelector        map[string]string `json:"metricsSelector,omitempty" yaml:"metricsSelector,omitempty"`
	Name                   string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId            string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences        []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Priority               int64             `json:"priority,omitempty" yaml:"priority,omitempty"`
	ProjectID              string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed                string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	ResourceType           string            `json:"resourceType,omitempty" yaml:"resourceType,omitempty"`
	UUID                   string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	YAxis                  *YAxis            `json:"yAxis,omitempty" yaml:"yAxis,omitempty"`
}

type ProjectMonitorGraphCollection struct {
	types.Collection
	Data   []ProjectMonitorGraph `json:"data,omitempty"`
	client *ProjectMonitorGraphClient
}

type ProjectMonitorGraphClient struct {
	apiClient *Client
}

type ProjectMonitorGraphOperations interface {
	List(opts *types.ListOpts) (*ProjectMonitorGraphCollection, error)
	ListAll(opts *types.ListOpts) (*ProjectMonitorGraphCollection, error)
	Create(opts *ProjectMonitorGraph) (*ProjectMonitorGraph, error)
	Update(existing *ProjectMonitorGraph, updates interface{}) (*ProjectMonitorGraph, error)
	Replace(existing *ProjectMonitorGraph) (*ProjectMonitorGraph, error)
	ByID(id string) (*ProjectMonitorGraph, error)
	Delete(container *ProjectMonitorGraph) error

	CollectionActionQuery(resource *ProjectMonitorGraphCollection, input *QueryGraphInput) (*QueryProjectGraphOutput, error)
}

func newProjectMonitorGraphClient(apiClient *Client) *ProjectMonitorGraphClient {
	return &ProjectMonitorGraphClient{
		apiClient: apiClient,
	}
}

func (c *ProjectMonitorGraphClient) Create(container *ProjectMonitorGraph) (*ProjectMonitorGraph, error) {
	resp := &ProjectMonitorGraph{}
	err := c.apiClient.Ops.DoCreate(ProjectMonitorGraphType, container, resp)
	return resp, err
}

func (c *ProjectMonitorGraphClient) Update(existing *ProjectMonitorGraph, updates interface{}) (*ProjectMonitorGraph, error) {
	resp := &ProjectMonitorGraph{}
	err := c.apiClient.Ops.DoUpdate(ProjectMonitorGraphType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ProjectMonitorGraphClient) Replace(obj *ProjectMonitorGraph) (*ProjectMonitorGraph, error) {
	resp := &ProjectMonitorGraph{}
	err := c.apiClient.Ops.DoReplace(ProjectMonitorGraphType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ProjectMonitorGraphClient) List(opts *types.ListOpts) (*ProjectMonitorGraphCollection, error) {
	resp := &ProjectMonitorGraphCollection{}
	err := c.apiClient.Ops.DoList(ProjectMonitorGraphType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ProjectMonitorGraphClient) ListAll(opts *types.ListOpts) (*ProjectMonitorGraphCollection, error) {
	resp := &ProjectMonitorGraphCollection{}
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

func (cc *ProjectMonitorGraphCollection) Next() (*ProjectMonitorGraphCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ProjectMonitorGraphCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ProjectMonitorGraphClient) ByID(id string) (*ProjectMonitorGraph, error) {
	resp := &ProjectMonitorGraph{}
	err := c.apiClient.Ops.DoByID(ProjectMonitorGraphType, id, resp)
	return resp, err
}

func (c *ProjectMonitorGraphClient) Delete(container *ProjectMonitorGraph) error {
	return c.apiClient.Ops.DoResourceDelete(ProjectMonitorGraphType, &container.Resource)
}

func (c *ProjectMonitorGraphClient) CollectionActionQuery(resource *ProjectMonitorGraphCollection, input *QueryGraphInput) (*QueryProjectGraphOutput, error) {
	resp := &QueryProjectGraphOutput{}
	err := c.apiClient.Ops.DoCollectionAction(ProjectMonitorGraphType, "query", &resource.Collection, input, resp)
	return resp, err
}
