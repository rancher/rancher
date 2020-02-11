package writer

import (
	"io"
	"net/http"
	"strconv"

	"github.com/rancher/steve/pkg/schemaserver/types"
)

type EncodingResponseWriter struct {
	ContentType string
	Encoder     func(io.Writer, interface{}) error
}

func (j *EncodingResponseWriter) start(apiOp *types.APIRequest, code int) {
	AddCommonResponseHeader(apiOp)
	apiOp.Response.Header().Set("content-type", j.ContentType)
	apiOp.Response.WriteHeader(code)
}

func (j *EncodingResponseWriter) Write(apiOp *types.APIRequest, code int, obj types.APIObject) {
	j.start(apiOp, code)
	j.Body(apiOp, apiOp.Response, obj)
}

func (j *EncodingResponseWriter) WriteList(apiOp *types.APIRequest, code int, list types.APIObjectList) {
	j.start(apiOp, code)
	j.BodyList(apiOp, apiOp.Response, list)
}

func (j *EncodingResponseWriter) Body(apiOp *types.APIRequest, writer io.Writer, obj types.APIObject) error {
	return j.Encoder(writer, j.convert(apiOp, obj))
}

func (j *EncodingResponseWriter) BodyList(apiOp *types.APIRequest, writer io.Writer, list types.APIObjectList) error {
	return j.Encoder(writer, j.convertList(apiOp, list))
}

func (j *EncodingResponseWriter) convertList(apiOp *types.APIRequest, input types.APIObjectList) *types.GenericCollection {
	collection := newCollection(apiOp, input)
	for _, value := range input.Objects {
		converted := j.convert(apiOp, value)
		collection.Data = append(collection.Data, converted)
	}

	if apiOp.Schema.CollectionFormatter != nil {
		apiOp.Schema.CollectionFormatter(apiOp, collection)
	}

	if collection.Data == nil {
		collection.Data = []*types.RawResource{}
	}

	return collection
}

func (j *EncodingResponseWriter) convert(context *types.APIRequest, input types.APIObject) *types.RawResource {
	schema := context.Schemas.LookupSchema(input.Type)
	if schema == nil {
		schema = context.Schema
	}
	if schema == nil {
		return nil
	}

	rawResource := &types.RawResource{
		ID:          input.ID,
		Type:        schema.ID,
		Schema:      schema,
		Links:       map[string]string{},
		Actions:     map[string]string{},
		ActionLinks: context.Request.Header.Get("X-API-Action-Links") != "",
		APIObject:   input,
	}

	j.addLinks(schema, context, input, rawResource)

	if schema.Formatter != nil {
		schema.Formatter(context, rawResource)
	}

	return rawResource
}

func (j *EncodingResponseWriter) addLinks(schema *types.APISchema, context *types.APIRequest, input types.APIObject, rawResource *types.RawResource) {
	if rawResource.ID == "" {
		return
	}

	self := context.URLBuilder.ResourceLink(rawResource.Schema, rawResource.ID)
	if _, ok := rawResource.Links["self"]; !ok {
		rawResource.Links["self"] = self
	}
	if _, ok := rawResource.Links["update"]; !ok {
		if context.AccessControl.CanUpdate(context, input, schema) == nil {
			rawResource.Links["update"] = self
		}
	}
	if _, ok := rawResource.Links["remove"]; !ok {
		if context.AccessControl.CanDelete(context, input, schema) == nil {
			rawResource.Links["remove"] = self
		}
	}
}

func getLimit(req *http.Request) int {
	limit, err := strconv.Atoi(req.Header.Get("limit"))
	if err == nil && limit > 0 {
		return limit
	}
	return 0
}

func newCollection(apiOp *types.APIRequest, list types.APIObjectList) *types.GenericCollection {
	result := &types.GenericCollection{
		Collection: types.Collection{
			Type:         "collection",
			ResourceType: apiOp.Type,
			CreateTypes:  map[string]string{},
			Links: map[string]string{
				"self": apiOp.URLBuilder.Current(),
			},
			Actions:  map[string]string{},
			Continue: list.Continue,
			Revision: list.Revision,
		},
	}

	partial := list.Continue != "" || apiOp.Query.Get("continue") != ""
	if partial {
		result.Pagination = &types.Pagination{
			Limit:   getLimit(apiOp.Request),
			First:   apiOp.URLBuilder.Current(),
			Partial: true,
		}
		if list.Continue != "" {
			result.Pagination.Next = apiOp.URLBuilder.Marker(list.Continue)
		}
	}

	if apiOp.Method == http.MethodGet {
		if apiOp.AccessControl.CanCreate(apiOp, apiOp.Schema) == nil {
			result.CreateTypes[apiOp.Schema.ID] = apiOp.URLBuilder.Collection(apiOp.Schema)
		}
	}

	return result
}
