package types

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	meta2 "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type RawResource struct {
	ID          string            `json:"id,omitempty" yaml:"id,omitempty"`
	Type        string            `json:"type,omitempty" yaml:"type,omitempty"`
	Schema      *APISchema        `json:"-" yaml:"-"`
	Links       map[string]string `json:"links" yaml:"links,omitempty"`
	Actions     map[string]string `json:"actions,omitempty" yaml:"actions,omitempty"`
	ActionLinks bool              `json:"-" yaml:"-"`
	APIObject   APIObject         `json:"-" yaml:"-"`
}

type Pagination struct {
	Limit   int    `json:"limit,omitempty"`
	First   string `json:"first,omitempty"`
	Next    string `json:"next,omitempty"`
	Partial bool   `json:"partial,omitempty"`
}

func (r *RawResource) MarshalJSON() ([]byte, error) {
	type r_ RawResource
	outer, err := json.Marshal((*r_)(r))
	if err != nil {
		return nil, err
	}

	last := len(outer) - 1
	if len(outer) < 2 || outer[last] != '}' {
		return outer, nil
	}

	data, err := json.Marshal(r.APIObject.Object)
	if err != nil {
		return nil, err
	}

	if len(data) < 3 || data[0] != '{' || data[len(data)-1] != '}' {
		return outer, nil
	}

	if outer[last-1] == '{' {
		outer[last] = ' '
	} else {
		outer[last] = ','
	}

	return append(outer, data[1:]...), nil
}

func (r *RawResource) AddAction(apiOp *APIRequest, name string) {
	r.Actions[name] = apiOp.URLBuilder.Action(r.Schema, r.ID, name)
}

type RequestHandler func(request *APIRequest) (APIObject, error)

type RequestListHandler func(request *APIRequest) (APIObjectList, error)

type Formatter func(request *APIRequest, resource *RawResource)

type RequestModifier func(request *APIRequest, schema *APISchema) *APISchema

type CollectionFormatter func(request *APIRequest, collection *GenericCollection)

type ErrorHandler func(request *APIRequest, err error)

type ResponseWriter interface {
	Write(apiOp *APIRequest, code int, obj APIObject)
	WriteList(apiOp *APIRequest, code int, obj APIObjectList)
}

type AccessControl interface {
	CanAction(apiOp *APIRequest, schema *APISchema, name string) error
	CanCreate(apiOp *APIRequest, schema *APISchema) error
	CanList(apiOp *APIRequest, schema *APISchema) error
	CanGet(apiOp *APIRequest, schema *APISchema) error
	CanUpdate(apiOp *APIRequest, obj APIObject, schema *APISchema) error
	CanDelete(apiOp *APIRequest, obj APIObject, schema *APISchema) error
	CanWatch(apiOp *APIRequest, schema *APISchema) error
}

type APIRequest struct {
	Action         string
	Name           string
	Type           string
	Link           string
	Method         string
	Namespace      string
	Schema         *APISchema
	Schemas        *APISchemas
	Query          url.Values
	ResponseFormat string
	ResponseWriter ResponseWriter
	URLPrefix      string
	URLBuilder     URLBuilder
	AccessControl  AccessControl

	Request  *http.Request
	Response http.ResponseWriter
}

type apiOpKey struct{}

func GetAPIContext(ctx context.Context) *APIRequest {
	apiOp, _ := ctx.Value(apiOpKey{}).(*APIRequest)
	return apiOp
}

func StoreAPIContext(apiOp *APIRequest) *APIRequest {
	ctx := context.WithValue(apiOp.Request.Context(), apiOpKey{}, apiOp)
	apiOp.Request = apiOp.Request.WithContext(ctx)
	return apiOp
}

func (r *APIRequest) WithContext(ctx context.Context) *APIRequest {
	result := *r
	result.Request = result.Request.WithContext(ctx)
	return &result
}

func (r *APIRequest) Context() context.Context {
	return r.Request.Context()
}

func (r *APIRequest) GetUser() string {
	user, ok := request.UserFrom(r.Request.Context())
	if ok {
		return user.GetName()
	}
	return ""
}

func (r *APIRequest) GetUserInfo() (user.Info, bool) {
	return request.UserFrom(r.Request.Context())
}

func (r *APIRequest) Option(key string) string {
	return r.Query.Get("_" + key)
}

func (r *APIRequest) WriteResponse(code int, obj APIObject) {
	r.ResponseWriter.Write(r, code, obj)
}

func (r *APIRequest) WriteResponseList(code int, list APIObjectList) {
	r.ResponseWriter.WriteList(r, code, list)
}

type URLBuilder interface {
	Current() string

	Collection(schema *APISchema) string
	CollectionAction(schema *APISchema, action string) string
	ResourceLink(schema *APISchema, id string) string
	Link(schema *APISchema, id string, linkName string) string
	Action(schema *APISchema, id string, action string) string
	Marker(marker string) string

	RelativeToRoot(path string) string
}

type Store interface {
	ByID(apiOp *APIRequest, schema *APISchema, id string) (APIObject, error)
	List(apiOp *APIRequest, schema *APISchema) (APIObjectList, error)
	Create(apiOp *APIRequest, schema *APISchema, data APIObject) (APIObject, error)
	Update(apiOp *APIRequest, schema *APISchema, data APIObject, id string) (APIObject, error)
	Delete(apiOp *APIRequest, schema *APISchema, id string) (APIObject, error)
	Watch(apiOp *APIRequest, schema *APISchema, w WatchRequest) (chan APIEvent, error)
}

func DefaultByID(store Store, apiOp *APIRequest, schema *APISchema, id string) (APIObject, error) {
	list, err := store.List(apiOp, schema)
	if err != nil {
		return APIObject{}, err
	}

	for _, item := range list.Objects {
		if item.ID == id {
			return item, nil
		}
	}

	return APIObject{}, validation.NotFound
}

type WatchRequest struct {
	Revision string
	ID       string
	Selector string
}

var (
	ChangeAPIEvent = "resource.change"
	RemoveAPIEvent = "resource.remove"
	CreateAPIEvent = "resource.create"
)

type APIEvent struct {
	Name         string    `json:"name,omitempty"`
	ResourceType string    `json:"resourceType,omitempty"`
	ID           string    `json:"id,omitempty"`
	Selector     string    `json:"selector,omitempty"`
	Revision     string    `json:"revision,omitempty"`
	Object       APIObject `json:"-"`
	Error        error     `json:"-"`
	// Data is the output format of the object
	Data interface{} `json:"data,omitempty"`
}

type APIObject struct {
	Type   string
	ID     string
	Object interface{}
}

type APIObjectList struct {
	Revision string
	Continue string
	Objects  []APIObject
}

func (a *APIObject) Data() data.Object {
	if unstr, ok := a.Object.(*unstructured.Unstructured); ok {
		return unstr.Object
	}
	data, err := convert.EncodeToMap(a.Object)
	if err != nil {
		return convert.ToMapInterface(a.Object)
	}
	return data
}

func (a *APIObject) Name() string {
	if ro, ok := a.Object.(runtime.Object); ok {
		meta, err := meta2.Accessor(ro)
		if err == nil {
			return meta.GetName()
		}
	}
	return Name(a.Data())
}

func (a *APIObject) Namespace() string {
	if ro, ok := a.Object.(runtime.Object); ok {
		meta, err := meta2.Accessor(ro)
		if err == nil {
			return meta.GetNamespace()
		}
	}
	return Namespace(a.Data())
}

func Name(d map[string]interface{}) string {
	return convert.ToString(data.GetValueN(d, "metadata", "name"))
}

func Namespace(d map[string]interface{}) string {
	return convert.ToString(data.GetValueN(d, "metadata", "namespace"))
}

func APIChan(c <-chan APIEvent, f func(APIObject) APIObject) chan APIEvent {
	if c == nil {
		return nil
	}
	result := make(chan APIEvent)
	go func() {
		for data := range c {
			data.Object = f(data.Object)
			result <- data
		}
		close(result)
	}()
	return result
}

func FormatterChain(formatter Formatter, next Formatter) Formatter {
	return func(request *APIRequest, resource *RawResource) {
		formatter(request, resource)
		next(request, resource)
	}
}

func (r *APIRequest) Clone() *APIRequest {
	clone := *r
	return &clone
}
