package client

import (
	"github.com/rancher/norman/types"
)

const (
	EndpointType                 = "endpoint"
	EndpointFieldAnnotations     = "annotations"
	EndpointFieldCreated         = "created"
	EndpointFieldFinalizers      = "finalizers"
	EndpointFieldLabels          = "labels"
	EndpointFieldName            = "name"
	EndpointFieldNamespaceId     = "namespaceId"
	EndpointFieldOwnerReferences = "ownerReferences"
	EndpointFieldPodIDs          = "podIds"
	EndpointFieldProjectID       = "projectId"
	EndpointFieldRemoved         = "removed"
	EndpointFieldTargets         = "targets"
	EndpointFieldUuid            = "uuid"
)

type Endpoint struct {
	types.Resource
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	PodIDs          []string          `json:"podIds,omitempty"`
	ProjectID       string            `json:"projectId,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Targets         []Target          `json:"targets,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
type EndpointCollection struct {
	types.Collection
	Data   []Endpoint `json:"data,omitempty"`
	client *EndpointClient
}

type EndpointClient struct {
	apiClient *Client
}

type EndpointOperations interface {
	List(opts *types.ListOpts) (*EndpointCollection, error)
	Create(opts *Endpoint) (*Endpoint, error)
	Update(existing *Endpoint, updates interface{}) (*Endpoint, error)
	ByID(id string) (*Endpoint, error)
	Delete(container *Endpoint) error
}

func newEndpointClient(apiClient *Client) *EndpointClient {
	return &EndpointClient{
		apiClient: apiClient,
	}
}

func (c *EndpointClient) Create(container *Endpoint) (*Endpoint, error) {
	resp := &Endpoint{}
	err := c.apiClient.Ops.DoCreate(EndpointType, container, resp)
	return resp, err
}

func (c *EndpointClient) Update(existing *Endpoint, updates interface{}) (*Endpoint, error) {
	resp := &Endpoint{}
	err := c.apiClient.Ops.DoUpdate(EndpointType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *EndpointClient) List(opts *types.ListOpts) (*EndpointCollection, error) {
	resp := &EndpointCollection{}
	err := c.apiClient.Ops.DoList(EndpointType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *EndpointCollection) Next() (*EndpointCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &EndpointCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *EndpointClient) ByID(id string) (*Endpoint, error) {
	resp := &Endpoint{}
	err := c.apiClient.Ops.DoByID(EndpointType, id, resp)
	return resp, err
}

func (c *EndpointClient) Delete(container *Endpoint) error {
	return c.apiClient.Ops.DoResourceDelete(EndpointType, &container.Resource)
}
