package types

import (
	"encoding/json"
	"net/http"
	"net/url"
)

type ValuesMap struct {
	Foo map[string]interface{}
}

type RawResource struct {
	ID          string                 `json:"id,omitempty" yaml:"id,omitempty"`
	Type        string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Schema      *Schema                `json:"-" yaml:"-"`
	Links       map[string]string      `json:"links" yaml:"links"`
	Actions     map[string]string      `json:"actions" yaml:"actions"`
	Values      map[string]interface{} `json:",inline"`
	ActionLinks bool                   `json:"-"`
}

func (r *RawResource) AddAction(apiContext *APIContext, name string) {
	r.Actions[name] = apiContext.URLBuilder.Action(name, r)
}

func (r *RawResource) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{}
	for k, v := range r.Values {
		data[k] = v
	}
	if r.ID != "" {
		data["id"] = r.ID
	}

	data["type"] = r.Type
	data["baseType"] = r.Schema.BaseType
	data["links"] = r.Links
	if r.ActionLinks {
		data["actionLinks"] = r.Actions
	} else {
		data["actions"] = r.Actions
	}
	return json.Marshal(data)
}

type ActionHandler func(actionName string, action *Action, request *APIContext) error

type RequestHandler func(request *APIContext) error

type QueryFilter func(opts *QueryOptions, data []map[string]interface{}) []map[string]interface{}

type Validator func(request *APIContext, data map[string]interface{}) error

type Formatter func(request *APIContext, resource *RawResource)

type ErrorHandler func(request *APIContext, err error)

type SubContextAttributeProvider interface {
	Query(apiContext *APIContext, schema *Schema) []*QueryCondition
	Create(apiContext *APIContext, schema *Schema) map[string]interface{}
}

type ResponseWriter interface {
	Write(apiContext *APIContext, code int, obj interface{})
}

type AccessControl interface {
	CanCreate(apiContext *APIContext, schema *Schema) bool
	CanList(apiContext *APIContext, schema *Schema) bool
	CanUpdate(apiContext *APIContext, obj map[string]interface{}, schema *Schema) bool
	CanDelete(apiContext *APIContext, obj map[string]interface{}, schema *Schema) bool

	Filter(apiContext *APIContext, obj map[string]interface{}, context map[string]string) map[string]interface{}
	FilterList(apiContext *APIContext, obj []map[string]interface{}, context map[string]string) []map[string]interface{}
}

type APIContext struct {
	Action                      string
	ID                          string
	Type                        string
	Link                        string
	Method                      string
	Schema                      *Schema
	Schemas                     *Schemas
	Version                     *APIVersion
	Query                       url.Values
	ResponseFormat              string
	ReferenceValidator          ReferenceValidator
	ResponseWriter              ResponseWriter
	QueryFilter                 QueryFilter
	SubContextAttributeProvider SubContextAttributeProvider
	//QueryOptions                *QueryOptions
	URLBuilder    URLBuilder
	AccessControl AccessControl
	SubContext    map[string]string
	//Attributes    map[string]interface{}

	Request  *http.Request
	Response http.ResponseWriter
}

func (r *APIContext) WriteResponse(code int, obj interface{}) {
	r.ResponseWriter.Write(r, code, obj)
}

func (r *APIContext) FilterList(opts *QueryOptions, obj []map[string]interface{}) []map[string]interface{} {
	return r.QueryFilter(opts, obj)
}

func (r *APIContext) FilterObject(opts *QueryOptions, obj map[string]interface{}) map[string]interface{} {
	opts.Pagination = nil
	result := r.QueryFilter(opts, []map[string]interface{}{obj})
	if len(result) == 0 {
		return nil
	}
	return result[0]
}

func (r *APIContext) Filter(opts *QueryOptions, obj interface{}) interface{} {
	switch v := obj.(type) {
	case []map[string]interface{}:
		return r.FilterList(opts, v)
	case map[string]interface{}:
		return r.FilterObject(opts, v)
	}

	return nil
}

var (
	ASC  = SortOrder("asc")
	DESC = SortOrder("desc")
)

type QueryOptions struct {
	Sort       Sort
	Pagination *Pagination
	Conditions []*QueryCondition
	Options    map[string]string
}

type ReferenceValidator interface {
	Validate(resourceType, resourceID string) bool
	Lookup(resourceType, resourceID string) *RawResource
}

type URLBuilder interface {
	Current() string
	Collection(schema *Schema, versionOverride *APIVersion) string
	SubContextCollection(subContext *Schema, contextName string, schema *Schema) string
	SchemaLink(schema *Schema) string
	ResourceLink(resource *RawResource) string
	Link(linkName string, resource *RawResource) string
	RelativeToRoot(path string) string
	Version(version APIVersion) string
	Marker(marker string) string
	ReverseSort(order SortOrder) string
	Sort(field string) string
	SetSubContext(subContext string)
	FilterLink(schema *Schema, fieldName string, value string) string
	Action(action string, resource *RawResource) string
	ResourceLinkByID(schema *Schema, id string) string
}

type Store interface {
	ByID(apiContext *APIContext, schema *Schema, id string) (map[string]interface{}, error)
	List(apiContext *APIContext, schema *Schema, opt *QueryOptions) ([]map[string]interface{}, error)
	Create(apiContext *APIContext, schema *Schema, data map[string]interface{}) (map[string]interface{}, error)
	Update(apiContext *APIContext, schema *Schema, data map[string]interface{}, id string) (map[string]interface{}, error)
	Delete(apiContext *APIContext, schema *Schema, id string) (map[string]interface{}, error)
	Watch(apiContext *APIContext, schema *Schema, opt *QueryOptions) (chan map[string]interface{}, error)
}
