package client

import (
	"github.com/rancher/norman/types"
)

const (
	NamespaceType                               = "namespace"
	NamespaceFieldAnnotations                   = "annotations"
	NamespaceFieldContainerDefaultResourceLimit = "containerDefaultResourceLimit"
	NamespaceFieldCreated                       = "created"
	NamespaceFieldCreatorID                     = "creatorId"
	NamespaceFieldDescription                   = "description"
	NamespaceFieldLabels                        = "labels"
	NamespaceFieldName                          = "name"
	NamespaceFieldOwnerReferences               = "ownerReferences"
	NamespaceFieldProjectID                     = "projectId"
	NamespaceFieldRemoved                       = "removed"
	NamespaceFieldResourceQuota                 = "resourceQuota"
	NamespaceFieldState                         = "state"
	NamespaceFieldTransitioning                 = "transitioning"
	NamespaceFieldTransitioningMessage          = "transitioningMessage"
	NamespaceFieldUUID                          = "uuid"
)

type Namespace struct {
	types.Resource
	Annotations                   map[string]string       `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ContainerDefaultResourceLimit *ContainerResourceLimit `json:"containerDefaultResourceLimit,omitempty" yaml:"containerDefaultResourceLimit,omitempty"`
	Created                       string                  `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                  `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description                   string                  `json:"description,omitempty" yaml:"description,omitempty"`
	Labels                        map[string]string       `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                          string                  `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences               []OwnerReference        `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID                     string                  `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed                       string                  `json:"removed,omitempty" yaml:"removed,omitempty"`
	ResourceQuota                 *NamespaceResourceQuota `json:"resourceQuota,omitempty" yaml:"resourceQuota,omitempty"`
	State                         string                  `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning                 string                  `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage          string                  `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                          string                  `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type NamespaceCollection struct {
	types.Collection
	Data   []Namespace `json:"data,omitempty"`
	client *NamespaceClient
}

type NamespaceClient struct {
	apiClient *Client
}

type NamespaceOperations interface {
	List(opts *types.ListOpts) (*NamespaceCollection, error)
	ListAll(opts *types.ListOpts) (*NamespaceCollection, error)
	Create(opts *Namespace) (*Namespace, error)
	Update(existing *Namespace, updates interface{}) (*Namespace, error)
	Replace(existing *Namespace) (*Namespace, error)
	ByID(id string) (*Namespace, error)
	Delete(container *Namespace) error

	ActionMove(resource *Namespace, input *NamespaceMove) error
}

func newNamespaceClient(apiClient *Client) *NamespaceClient {
	return &NamespaceClient{
		apiClient: apiClient,
	}
}

func (c *NamespaceClient) Create(container *Namespace) (*Namespace, error) {
	resp := &Namespace{}
	err := c.apiClient.Ops.DoCreate(NamespaceType, container, resp)
	return resp, err
}

func (c *NamespaceClient) Update(existing *Namespace, updates interface{}) (*Namespace, error) {
	resp := &Namespace{}
	err := c.apiClient.Ops.DoUpdate(NamespaceType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NamespaceClient) Replace(obj *Namespace) (*Namespace, error) {
	resp := &Namespace{}
	err := c.apiClient.Ops.DoReplace(NamespaceType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *NamespaceClient) List(opts *types.ListOpts) (*NamespaceCollection, error) {
	resp := &NamespaceCollection{}
	err := c.apiClient.Ops.DoList(NamespaceType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *NamespaceClient) ListAll(opts *types.ListOpts) (*NamespaceCollection, error) {
	resp := &NamespaceCollection{}
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

func (cc *NamespaceCollection) Next() (*NamespaceCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NamespaceCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NamespaceClient) ByID(id string) (*Namespace, error) {
	resp := &Namespace{}
	err := c.apiClient.Ops.DoByID(NamespaceType, id, resp)
	return resp, err
}

func (c *NamespaceClient) Delete(container *Namespace) error {
	return c.apiClient.Ops.DoResourceDelete(NamespaceType, &container.Resource)
}

func (c *NamespaceClient) ActionMove(resource *Namespace, input *NamespaceMove) error {
	err := c.apiClient.Ops.DoAction(NamespaceType, "move", &resource.Resource, input, nil)
	return err
}
