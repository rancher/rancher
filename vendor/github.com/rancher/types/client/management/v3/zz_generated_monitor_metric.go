package client

import (
	"github.com/rancher/norman/types"
)

const (
	MonitorMetricType                 = "monitorMetric"
	MonitorMetricFieldAnnotations     = "annotations"
	MonitorMetricFieldCreated         = "created"
	MonitorMetricFieldCreatorID       = "creatorId"
	MonitorMetricFieldDescription     = "description"
	MonitorMetricFieldExpression      = "expression"
	MonitorMetricFieldLabels          = "labels"
	MonitorMetricFieldLegendFormat    = "legendFormat"
	MonitorMetricFieldName            = "name"
	MonitorMetricFieldNamespaceId     = "namespaceId"
	MonitorMetricFieldOwnerReferences = "ownerReferences"
	MonitorMetricFieldRemoved         = "removed"
	MonitorMetricFieldUUID            = "uuid"
)

type MonitorMetric struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
	Expression      string            `json:"expression,omitempty" yaml:"expression,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LegendFormat    string            `json:"legendFormat,omitempty" yaml:"legendFormat,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type MonitorMetricCollection struct {
	types.Collection
	Data   []MonitorMetric `json:"data,omitempty"`
	client *MonitorMetricClient
}

type MonitorMetricClient struct {
	apiClient *Client
}

type MonitorMetricOperations interface {
	List(opts *types.ListOpts) (*MonitorMetricCollection, error)
	ListAll(opts *types.ListOpts) (*MonitorMetricCollection, error)
	Create(opts *MonitorMetric) (*MonitorMetric, error)
	Update(existing *MonitorMetric, updates interface{}) (*MonitorMetric, error)
	Replace(existing *MonitorMetric) (*MonitorMetric, error)
	ByID(id string) (*MonitorMetric, error)
	Delete(container *MonitorMetric) error

	CollectionActionListclustermetricname(resource *MonitorMetricCollection, input *ClusterMetricNamesInput) (*MetricNamesOutput, error)

	CollectionActionListprojectmetricname(resource *MonitorMetricCollection, input *ProjectMetricNamesInput) (*MetricNamesOutput, error)

	CollectionActionQuerycluster(resource *MonitorMetricCollection, input *QueryClusterMetricInput) (*QueryMetricOutput, error)

	CollectionActionQueryproject(resource *MonitorMetricCollection, input *QueryProjectMetricInput) (*QueryMetricOutput, error)
}

func newMonitorMetricClient(apiClient *Client) *MonitorMetricClient {
	return &MonitorMetricClient{
		apiClient: apiClient,
	}
}

func (c *MonitorMetricClient) Create(container *MonitorMetric) (*MonitorMetric, error) {
	resp := &MonitorMetric{}
	err := c.apiClient.Ops.DoCreate(MonitorMetricType, container, resp)
	return resp, err
}

func (c *MonitorMetricClient) Update(existing *MonitorMetric, updates interface{}) (*MonitorMetric, error) {
	resp := &MonitorMetric{}
	err := c.apiClient.Ops.DoUpdate(MonitorMetricType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *MonitorMetricClient) Replace(obj *MonitorMetric) (*MonitorMetric, error) {
	resp := &MonitorMetric{}
	err := c.apiClient.Ops.DoReplace(MonitorMetricType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *MonitorMetricClient) List(opts *types.ListOpts) (*MonitorMetricCollection, error) {
	resp := &MonitorMetricCollection{}
	err := c.apiClient.Ops.DoList(MonitorMetricType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *MonitorMetricClient) ListAll(opts *types.ListOpts) (*MonitorMetricCollection, error) {
	resp := &MonitorMetricCollection{}
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

func (cc *MonitorMetricCollection) Next() (*MonitorMetricCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &MonitorMetricCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *MonitorMetricClient) ByID(id string) (*MonitorMetric, error) {
	resp := &MonitorMetric{}
	err := c.apiClient.Ops.DoByID(MonitorMetricType, id, resp)
	return resp, err
}

func (c *MonitorMetricClient) Delete(container *MonitorMetric) error {
	return c.apiClient.Ops.DoResourceDelete(MonitorMetricType, &container.Resource)
}

func (c *MonitorMetricClient) CollectionActionListclustermetricname(resource *MonitorMetricCollection, input *ClusterMetricNamesInput) (*MetricNamesOutput, error) {
	resp := &MetricNamesOutput{}
	err := c.apiClient.Ops.DoCollectionAction(MonitorMetricType, "listclustermetricname", &resource.Collection, input, resp)
	return resp, err
}

func (c *MonitorMetricClient) CollectionActionListprojectmetricname(resource *MonitorMetricCollection, input *ProjectMetricNamesInput) (*MetricNamesOutput, error) {
	resp := &MetricNamesOutput{}
	err := c.apiClient.Ops.DoCollectionAction(MonitorMetricType, "listprojectmetricname", &resource.Collection, input, resp)
	return resp, err
}

func (c *MonitorMetricClient) CollectionActionQuerycluster(resource *MonitorMetricCollection, input *QueryClusterMetricInput) (*QueryMetricOutput, error) {
	resp := &QueryMetricOutput{}
	err := c.apiClient.Ops.DoCollectionAction(MonitorMetricType, "querycluster", &resource.Collection, input, resp)
	return resp, err
}

func (c *MonitorMetricClient) CollectionActionQueryproject(resource *MonitorMetricCollection, input *QueryProjectMetricInput) (*QueryMetricOutput, error) {
	resp := &QueryMetricOutput{}
	err := c.apiClient.Ops.DoCollectionAction(MonitorMetricType, "queryproject", &resource.Collection, input, resp)
	return resp, err
}
