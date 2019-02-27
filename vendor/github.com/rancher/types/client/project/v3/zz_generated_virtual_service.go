package client

import (
	"github.com/rancher/norman/types"
)

const (
	VirtualServiceType                      = "virtualService"
	VirtualServiceFieldAnnotations          = "annotations"
	VirtualServiceFieldCreated              = "created"
	VirtualServiceFieldCreatorID            = "creatorId"
	VirtualServiceFieldGateways             = "gateways"
	VirtualServiceFieldHTTP                 = "http"
	VirtualServiceFieldHosts                = "hosts"
	VirtualServiceFieldLabels               = "labels"
	VirtualServiceFieldName                 = "name"
	VirtualServiceFieldNamespaceId          = "namespaceId"
	VirtualServiceFieldOwnerReferences      = "ownerReferences"
	VirtualServiceFieldProjectID            = "projectId"
	VirtualServiceFieldRemoved              = "removed"
	VirtualServiceFieldState                = "state"
	VirtualServiceFieldStatus               = "status"
	VirtualServiceFieldTCP                  = "tcp"
	VirtualServiceFieldTLS                  = "tls"
	VirtualServiceFieldTransitioning        = "transitioning"
	VirtualServiceFieldTransitioningMessage = "transitioningMessage"
	VirtualServiceFieldUUID                 = "uuid"
)

type VirtualService struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Gateways             []string          `json:"gateways,omitempty" yaml:"gateways,omitempty"`
	HTTP                 []HTTPRoute       `json:"http,omitempty" yaml:"http,omitempty"`
	Hosts                []string          `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID            string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               interface{}       `json:"status,omitempty" yaml:"status,omitempty"`
	TCP                  []TCPRoute        `json:"tcp,omitempty" yaml:"tcp,omitempty"`
	TLS                  []TLSRoute        `json:"tls,omitempty" yaml:"tls,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type VirtualServiceCollection struct {
	types.Collection
	Data   []VirtualService `json:"data,omitempty"`
	client *VirtualServiceClient
}

type VirtualServiceClient struct {
	apiClient *Client
}

type VirtualServiceOperations interface {
	List(opts *types.ListOpts) (*VirtualServiceCollection, error)
	Create(opts *VirtualService) (*VirtualService, error)
	Update(existing *VirtualService, updates interface{}) (*VirtualService, error)
	Replace(existing *VirtualService) (*VirtualService, error)
	ByID(id string) (*VirtualService, error)
	Delete(container *VirtualService) error
}

func newVirtualServiceClient(apiClient *Client) *VirtualServiceClient {
	return &VirtualServiceClient{
		apiClient: apiClient,
	}
}

func (c *VirtualServiceClient) Create(container *VirtualService) (*VirtualService, error) {
	resp := &VirtualService{}
	err := c.apiClient.Ops.DoCreate(VirtualServiceType, container, resp)
	return resp, err
}

func (c *VirtualServiceClient) Update(existing *VirtualService, updates interface{}) (*VirtualService, error) {
	resp := &VirtualService{}
	err := c.apiClient.Ops.DoUpdate(VirtualServiceType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *VirtualServiceClient) Replace(obj *VirtualService) (*VirtualService, error) {
	resp := &VirtualService{}
	err := c.apiClient.Ops.DoReplace(VirtualServiceType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *VirtualServiceClient) List(opts *types.ListOpts) (*VirtualServiceCollection, error) {
	resp := &VirtualServiceCollection{}
	err := c.apiClient.Ops.DoList(VirtualServiceType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *VirtualServiceCollection) Next() (*VirtualServiceCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &VirtualServiceCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *VirtualServiceClient) ByID(id string) (*VirtualService, error) {
	resp := &VirtualService{}
	err := c.apiClient.Ops.DoByID(VirtualServiceType, id, resp)
	return resp, err
}

func (c *VirtualServiceClient) Delete(container *VirtualService) error {
	return c.apiClient.Ops.DoResourceDelete(VirtualServiceType, &container.Resource)
}
