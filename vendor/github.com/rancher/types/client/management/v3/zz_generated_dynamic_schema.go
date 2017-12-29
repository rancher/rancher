package client

import (
	"github.com/rancher/norman/types"
)

const (
	DynamicSchemaType                      = "dynamicSchema"
	DynamicSchemaFieldAnnotations          = "annotations"
	DynamicSchemaFieldCollectionActions    = "collectionActions"
	DynamicSchemaFieldCollectionFields     = "collectionFields"
	DynamicSchemaFieldCollectionFilters    = "collectionFilters"
	DynamicSchemaFieldCollectionMethods    = "collectionMethods"
	DynamicSchemaFieldCreated              = "created"
	DynamicSchemaFieldCreatorID            = "creatorId"
	DynamicSchemaFieldEmbed                = "embed"
	DynamicSchemaFieldEmbedType            = "embedType"
	DynamicSchemaFieldFinalizers           = "finalizers"
	DynamicSchemaFieldIncludeableLinks     = "includeableLinks"
	DynamicSchemaFieldLabels               = "labels"
	DynamicSchemaFieldName                 = "name"
	DynamicSchemaFieldOwnerReferences      = "ownerReferences"
	DynamicSchemaFieldPluralName           = "pluralName"
	DynamicSchemaFieldRemoved              = "removed"
	DynamicSchemaFieldResourceActions      = "resourceActions"
	DynamicSchemaFieldResourceFields       = "resourceFields"
	DynamicSchemaFieldResourceMethods      = "resourceMethods"
	DynamicSchemaFieldState                = "state"
	DynamicSchemaFieldStatus               = "status"
	DynamicSchemaFieldTransitioning        = "transitioning"
	DynamicSchemaFieldTransitioningMessage = "transitioningMessage"
	DynamicSchemaFieldUuid                 = "uuid"
)

type DynamicSchema struct {
	types.Resource
	Annotations          map[string]string    `json:"annotations,omitempty"`
	CollectionActions    map[string]Action    `json:"collectionActions,omitempty"`
	CollectionFields     map[string]Field     `json:"collectionFields,omitempty"`
	CollectionFilters    map[string]Filter    `json:"collectionFilters,omitempty"`
	CollectionMethods    []string             `json:"collectionMethods,omitempty"`
	Created              string               `json:"created,omitempty"`
	CreatorID            string               `json:"creatorId,omitempty"`
	Embed                *bool                `json:"embed,omitempty"`
	EmbedType            string               `json:"embedType,omitempty"`
	Finalizers           []string             `json:"finalizers,omitempty"`
	IncludeableLinks     []string             `json:"includeableLinks,omitempty"`
	Labels               map[string]string    `json:"labels,omitempty"`
	Name                 string               `json:"name,omitempty"`
	OwnerReferences      []OwnerReference     `json:"ownerReferences,omitempty"`
	PluralName           string               `json:"pluralName,omitempty"`
	Removed              string               `json:"removed,omitempty"`
	ResourceActions      map[string]Action    `json:"resourceActions,omitempty"`
	ResourceFields       map[string]Field     `json:"resourceFields,omitempty"`
	ResourceMethods      []string             `json:"resourceMethods,omitempty"`
	State                string               `json:"state,omitempty"`
	Status               *DynamicSchemaStatus `json:"status,omitempty"`
	Transitioning        string               `json:"transitioning,omitempty"`
	TransitioningMessage string               `json:"transitioningMessage,omitempty"`
	Uuid                 string               `json:"uuid,omitempty"`
}
type DynamicSchemaCollection struct {
	types.Collection
	Data   []DynamicSchema `json:"data,omitempty"`
	client *DynamicSchemaClient
}

type DynamicSchemaClient struct {
	apiClient *Client
}

type DynamicSchemaOperations interface {
	List(opts *types.ListOpts) (*DynamicSchemaCollection, error)
	Create(opts *DynamicSchema) (*DynamicSchema, error)
	Update(existing *DynamicSchema, updates interface{}) (*DynamicSchema, error)
	ByID(id string) (*DynamicSchema, error)
	Delete(container *DynamicSchema) error
}

func newDynamicSchemaClient(apiClient *Client) *DynamicSchemaClient {
	return &DynamicSchemaClient{
		apiClient: apiClient,
	}
}

func (c *DynamicSchemaClient) Create(container *DynamicSchema) (*DynamicSchema, error) {
	resp := &DynamicSchema{}
	err := c.apiClient.Ops.DoCreate(DynamicSchemaType, container, resp)
	return resp, err
}

func (c *DynamicSchemaClient) Update(existing *DynamicSchema, updates interface{}) (*DynamicSchema, error) {
	resp := &DynamicSchema{}
	err := c.apiClient.Ops.DoUpdate(DynamicSchemaType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DynamicSchemaClient) List(opts *types.ListOpts) (*DynamicSchemaCollection, error) {
	resp := &DynamicSchemaCollection{}
	err := c.apiClient.Ops.DoList(DynamicSchemaType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *DynamicSchemaCollection) Next() (*DynamicSchemaCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &DynamicSchemaCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *DynamicSchemaClient) ByID(id string) (*DynamicSchema, error) {
	resp := &DynamicSchema{}
	err := c.apiClient.Ops.DoByID(DynamicSchemaType, id, resp)
	return resp, err
}

func (c *DynamicSchemaClient) Delete(container *DynamicSchema) error {
	return c.apiClient.Ops.DoResourceDelete(DynamicSchemaType, &container.Resource)
}
