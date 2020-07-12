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
	DynamicSchemaFieldDynamicSchemaVersion = "dynamicSchemaVersion"
	DynamicSchemaFieldEmbed                = "embed"
	DynamicSchemaFieldEmbedType            = "embedType"
	DynamicSchemaFieldIncludeableLinks     = "includeableLinks"
	DynamicSchemaFieldLabels               = "labels"
	DynamicSchemaFieldName                 = "name"
	DynamicSchemaFieldOwnerReferences      = "ownerReferences"
	DynamicSchemaFieldPluralName           = "pluralName"
	DynamicSchemaFieldRemoved              = "removed"
	DynamicSchemaFieldResourceActions      = "resourceActions"
	DynamicSchemaFieldResourceFields       = "resourceFields"
	DynamicSchemaFieldResourceMethods      = "resourceMethods"
	DynamicSchemaFieldSchemaName           = "schemaName"
	DynamicSchemaFieldState                = "state"
	DynamicSchemaFieldStatus               = "status"
	DynamicSchemaFieldTransitioning        = "transitioning"
	DynamicSchemaFieldTransitioningMessage = "transitioningMessage"
	DynamicSchemaFieldUUID                 = "uuid"
)

type DynamicSchema struct {
	types.Resource
	Annotations          map[string]string    `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	CollectionActions    map[string]Action    `json:"collectionActions,omitempty" yaml:"collectionActions,omitempty"`
	CollectionFields     map[string]Field     `json:"collectionFields,omitempty" yaml:"collectionFields,omitempty"`
	CollectionFilters    map[string]Filter    `json:"collectionFilters,omitempty" yaml:"collectionFilters,omitempty"`
	CollectionMethods    []string             `json:"collectionMethods,omitempty" yaml:"collectionMethods,omitempty"`
	Created              string               `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string               `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DynamicSchemaVersion string               `json:"dynamicSchemaVersion,omitempty" yaml:"dynamicSchemaVersion,omitempty"`
	Embed                bool                 `json:"embed,omitempty" yaml:"embed,omitempty"`
	EmbedType            string               `json:"embedType,omitempty" yaml:"embedType,omitempty"`
	IncludeableLinks     []string             `json:"includeableLinks,omitempty" yaml:"includeableLinks,omitempty"`
	Labels               map[string]string    `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string               `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference     `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PluralName           string               `json:"pluralName,omitempty" yaml:"pluralName,omitempty"`
	Removed              string               `json:"removed,omitempty" yaml:"removed,omitempty"`
	ResourceActions      map[string]Action    `json:"resourceActions,omitempty" yaml:"resourceActions,omitempty"`
	ResourceFields       map[string]Field     `json:"resourceFields,omitempty" yaml:"resourceFields,omitempty"`
	ResourceMethods      []string             `json:"resourceMethods,omitempty" yaml:"resourceMethods,omitempty"`
	SchemaName           string               `json:"schemaName,omitempty" yaml:"schemaName,omitempty"`
	State                string               `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *DynamicSchemaStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string               `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string               `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string               `json:"uuid,omitempty" yaml:"uuid,omitempty"`
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
	ListAll(opts *types.ListOpts) (*DynamicSchemaCollection, error)
	Create(opts *DynamicSchema) (*DynamicSchema, error)
	Update(existing *DynamicSchema, updates interface{}) (*DynamicSchema, error)
	Replace(existing *DynamicSchema) (*DynamicSchema, error)
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

func (c *DynamicSchemaClient) Replace(obj *DynamicSchema) (*DynamicSchema, error) {
	resp := &DynamicSchema{}
	err := c.apiClient.Ops.DoReplace(DynamicSchemaType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *DynamicSchemaClient) List(opts *types.ListOpts) (*DynamicSchemaCollection, error) {
	resp := &DynamicSchemaCollection{}
	err := c.apiClient.Ops.DoList(DynamicSchemaType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *DynamicSchemaClient) ListAll(opts *types.ListOpts) (*DynamicSchemaCollection, error) {
	resp := &DynamicSchemaCollection{}
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
