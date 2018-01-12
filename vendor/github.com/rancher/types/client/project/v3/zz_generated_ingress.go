package client

import (
	"github.com/rancher/norman/types"
)

const (
	IngressType                      = "ingress"
	IngressFieldAnnotations          = "annotations"
	IngressFieldCreated              = "created"
	IngressFieldCreatorID            = "creatorId"
	IngressFieldDefaultBackend       = "defaultBackend"
	IngressFieldDescription          = "description"
	IngressFieldLabels               = "labels"
	IngressFieldName                 = "name"
	IngressFieldNamespaceId          = "namespaceId"
	IngressFieldOwnerReferences      = "ownerReferences"
	IngressFieldProjectID            = "projectId"
	IngressFieldRemoved              = "removed"
	IngressFieldRules                = "rules"
	IngressFieldState                = "state"
	IngressFieldStatus               = "status"
	IngressFieldTLS                  = "tls"
	IngressFieldTransitioning        = "transitioning"
	IngressFieldTransitioningMessage = "transitioningMessage"
	IngressFieldUuid                 = "uuid"
)

type Ingress struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty"`
	Created              string            `json:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty"`
	DefaultBackend       *IngressBackend   `json:"defaultBackend,omitempty"`
	Description          string            `json:"description,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Name                 string            `json:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectID            string            `json:"projectId,omitempty"`
	Removed              string            `json:"removed,omitempty"`
	Rules                []IngressRule     `json:"rules,omitempty"`
	State                string            `json:"state,omitempty"`
	Status               *IngressStatus    `json:"status,omitempty"`
	TLS                  []IngressTLS      `json:"tls,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty"`
	Uuid                 string            `json:"uuid,omitempty"`
}
type IngressCollection struct {
	types.Collection
	Data   []Ingress `json:"data,omitempty"`
	client *IngressClient
}

type IngressClient struct {
	apiClient *Client
}

type IngressOperations interface {
	List(opts *types.ListOpts) (*IngressCollection, error)
	Create(opts *Ingress) (*Ingress, error)
	Update(existing *Ingress, updates interface{}) (*Ingress, error)
	ByID(id string) (*Ingress, error)
	Delete(container *Ingress) error
}

func newIngressClient(apiClient *Client) *IngressClient {
	return &IngressClient{
		apiClient: apiClient,
	}
}

func (c *IngressClient) Create(container *Ingress) (*Ingress, error) {
	resp := &Ingress{}
	err := c.apiClient.Ops.DoCreate(IngressType, container, resp)
	return resp, err
}

func (c *IngressClient) Update(existing *Ingress, updates interface{}) (*Ingress, error) {
	resp := &Ingress{}
	err := c.apiClient.Ops.DoUpdate(IngressType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *IngressClient) List(opts *types.ListOpts) (*IngressCollection, error) {
	resp := &IngressCollection{}
	err := c.apiClient.Ops.DoList(IngressType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *IngressCollection) Next() (*IngressCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &IngressCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *IngressClient) ByID(id string) (*Ingress, error) {
	resp := &Ingress{}
	err := c.apiClient.Ops.DoByID(IngressType, id, resp)
	return resp, err
}

func (c *IngressClient) Delete(container *Ingress) error {
	return c.apiClient.Ops.DoResourceDelete(IngressType, &container.Resource)
}
