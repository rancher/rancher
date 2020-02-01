package types

import (
	"net/http"

	"github.com/rancher/wrangler/pkg/schemas"
)

const (
	ResourceFieldID = "id"
)

type Collection struct {
	Type         string            `json:"type,omitempty"`
	Links        map[string]string `json:"links"`
	CreateTypes  map[string]string `json:"createTypes,omitempty"`
	Actions      map[string]string `json:"actions"`
	ResourceType string            `json:"resourceType"`
	Revision     string            `json:"revision,omitempty"`
	Continue     string            `json:"continue,omitempty"`
}

type GenericCollection struct {
	Collection
	Data []*RawResource `json:"data"`
}

var (
	ModifierEQ      ModifierType = "eq"
	ModifierNE      ModifierType = "ne"
	ModifierNull    ModifierType = "null"
	ModifierNotNull ModifierType = "notnull"
	ModifierIn      ModifierType = "in"
	ModifierNotIn   ModifierType = "notin"
)

type ModifierType string

type Condition struct {
	Modifier ModifierType `json:"modifier,omitempty"`
	Value    interface{}  `json:"value,omitempty"`
}

type Resource struct {
	ID      string            `json:"id,omitempty"`
	Type    string            `json:"type,omitempty"`
	Links   map[string]string `json:"links"`
	Actions map[string]string `json:"actions"`
}

type NamedResource struct {
	Resource
	Name        string `json:"name"`
	Description string `json:"description"`
}

type NamedResourceCollection struct {
	Collection
	Data []NamedResource `json:"data,omitempty"`
}

type APISchema struct {
	*schemas.Schema

	ActionHandlers      map[string]http.Handler `json:"-"`
	LinkHandlers        map[string]http.Handler `json:"-"`
	ListHandler         RequestListHandler      `json:"-"`
	ByIDHandler         RequestHandler          `json:"-"`
	CreateHandler       RequestHandler          `json:"-"`
	DeleteHandler       RequestHandler          `json:"-"`
	UpdateHandler       RequestHandler          `json:"-"`
	Formatter           Formatter               `json:"-"`
	CollectionFormatter CollectionFormatter     `json:"-"`
	ErrorHandler        ErrorHandler            `json:"-"`
	Store               Store                   `json:"-"`
}

func (a *APISchema) DeepCopy() *APISchema {
	r := *a
	r.Schema = r.Schema.DeepCopy()
	return &r
}

func (c *Collection) AddAction(apiOp *APIRequest, name string) {
	c.Actions[name] = apiOp.URLBuilder.CollectionAction(apiOp.Schema, name)
}
