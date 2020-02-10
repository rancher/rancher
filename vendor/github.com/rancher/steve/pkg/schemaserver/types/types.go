package types

import (
	"net/http"

	"github.com/rancher/wrangler/pkg/schemas"
)

type Collection struct {
	Type         string            `json:"type,omitempty"`
	Links        map[string]string `json:"links"`
	CreateTypes  map[string]string `json:"createTypes,omitempty"`
	Actions      map[string]string `json:"actions"`
	ResourceType string            `json:"resourceType"`
	Pagination   *Pagination       `json:"pagination,omitempty"`
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

var ReservedFields = map[string]bool{
	"id":      true,
	"type":    true,
	"links":   true,
	"actions": true,
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

func copyHandlers(m map[string]http.Handler) map[string]http.Handler {
	if m == nil {
		return nil
	}
	result := make(map[string]http.Handler, len(m))
	for k, v := range m {
		result[k] = v
	}

	return result
}
func (a *APISchema) DeepCopy() *APISchema {
	r := *a
	r.ActionHandlers = copyHandlers(a.ActionHandlers)
	r.LinkHandlers = copyHandlers(a.ActionHandlers)
	r.Schema = r.Schema.DeepCopy()
	return &r
}

func (c *Collection) AddAction(apiOp *APIRequest, name string) {
	c.Actions[name] = apiOp.URLBuilder.CollectionAction(apiOp.Schema, name)
}
