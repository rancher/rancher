package client

import (
	"github.com/rancher/norman/types"
)

const (
	MachineTemplateType                      = "machineTemplate"
	MachineTemplateFieldAnnotations          = "annotations"
	MachineTemplateFieldCreated              = "created"
	MachineTemplateFieldDescription          = "description"
	MachineTemplateFieldDriver               = "driver"
	MachineTemplateFieldFinalizers           = "finalizers"
	MachineTemplateFieldFlavorPrefix         = "flavorPrefix"
	MachineTemplateFieldId                   = "id"
	MachineTemplateFieldLabels               = "labels"
	MachineTemplateFieldName                 = "name"
	MachineTemplateFieldOwnerReferences      = "ownerReferences"
	MachineTemplateFieldPublicValues         = "publicValues"
	MachineTemplateFieldRemoved              = "removed"
	MachineTemplateFieldResourcePath         = "resourcePath"
	MachineTemplateFieldSecretName           = "secretName"
	MachineTemplateFieldSecretValues         = "secretValues"
	MachineTemplateFieldState                = "state"
	MachineTemplateFieldStatus               = "status"
	MachineTemplateFieldTransitioning        = "transitioning"
	MachineTemplateFieldTransitioningMessage = "transitioningMessage"
	MachineTemplateFieldUuid                 = "uuid"
)

type MachineTemplate struct {
	types.Resource
	Annotations          map[string]string      `json:"annotations,omitempty"`
	Created              string                 `json:"created,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Driver               string                 `json:"driver,omitempty"`
	Finalizers           []string               `json:"finalizers,omitempty"`
	FlavorPrefix         string                 `json:"flavorPrefix,omitempty"`
	Id                   string                 `json:"id,omitempty"`
	Labels               map[string]string      `json:"labels,omitempty"`
	Name                 string                 `json:"name,omitempty"`
	OwnerReferences      []OwnerReference       `json:"ownerReferences,omitempty"`
	PublicValues         map[string]string      `json:"publicValues,omitempty"`
	Removed              string                 `json:"removed,omitempty"`
	ResourcePath         string                 `json:"resourcePath,omitempty"`
	SecretName           string                 `json:"secretName,omitempty"`
	SecretValues         map[string]string      `json:"secretValues,omitempty"`
	State                string                 `json:"state,omitempty"`
	Status               *MachineTemplateStatus `json:"status,omitempty"`
	Transitioning        string                 `json:"transitioning,omitempty"`
	TransitioningMessage string                 `json:"transitioningMessage,omitempty"`
	Uuid                 string                 `json:"uuid,omitempty"`
}
type MachineTemplateCollection struct {
	types.Collection
	Data   []MachineTemplate `json:"data,omitempty"`
	client *MachineTemplateClient
}

type MachineTemplateClient struct {
	apiClient *Client
}

type MachineTemplateOperations interface {
	List(opts *types.ListOpts) (*MachineTemplateCollection, error)
	Create(opts *MachineTemplate) (*MachineTemplate, error)
	Update(existing *MachineTemplate, updates interface{}) (*MachineTemplate, error)
	ByID(id string) (*MachineTemplate, error)
	Delete(container *MachineTemplate) error
}

func newMachineTemplateClient(apiClient *Client) *MachineTemplateClient {
	return &MachineTemplateClient{
		apiClient: apiClient,
	}
}

func (c *MachineTemplateClient) Create(container *MachineTemplate) (*MachineTemplate, error) {
	resp := &MachineTemplate{}
	err := c.apiClient.Ops.DoCreate(MachineTemplateType, container, resp)
	return resp, err
}

func (c *MachineTemplateClient) Update(existing *MachineTemplate, updates interface{}) (*MachineTemplate, error) {
	resp := &MachineTemplate{}
	err := c.apiClient.Ops.DoUpdate(MachineTemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *MachineTemplateClient) List(opts *types.ListOpts) (*MachineTemplateCollection, error) {
	resp := &MachineTemplateCollection{}
	err := c.apiClient.Ops.DoList(MachineTemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *MachineTemplateCollection) Next() (*MachineTemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &MachineTemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *MachineTemplateClient) ByID(id string) (*MachineTemplate, error) {
	resp := &MachineTemplate{}
	err := c.apiClient.Ops.DoByID(MachineTemplateType, id, resp)
	return resp, err
}

func (c *MachineTemplateClient) Delete(container *MachineTemplate) error {
	return c.apiClient.Ops.DoResourceDelete(MachineTemplateType, &container.Resource)
}
