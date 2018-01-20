package writer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/parse/builder"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
	"github.com/sirupsen/logrus"
)

type JSONResponseWriter struct {
}

func (j *JSONResponseWriter) start(apiContext *types.APIContext, code int, obj interface{}) {
	AddCommonResponseHeader(apiContext)
	apiContext.Response.Header().Set("content-type", "application/json")
	apiContext.Response.WriteHeader(code)
}

func (j *JSONResponseWriter) Write(apiContext *types.APIContext, code int, obj interface{}) {
	j.start(apiContext, code, obj)
	j.Body(apiContext, apiContext.Response, obj)
}

func (j *JSONResponseWriter) Body(apiContext *types.APIContext, writer io.Writer, obj interface{}) error {
	return j.VersionBody(apiContext, apiContext.Version, writer, obj)

}

func (j *JSONResponseWriter) VersionBody(apiContext *types.APIContext, version *types.APIVersion, writer io.Writer, obj interface{}) error {
	var output interface{}

	builder := builder.NewBuilder(apiContext)
	builder.Version = version

	switch v := obj.(type) {
	case []interface{}:
		output = j.writeInterfaceSlice(builder, apiContext, v)
	case []map[string]interface{}:
		output = j.writeMapSlice(builder, apiContext, v)
	case map[string]interface{}:
		output = j.convert(builder, apiContext, v)
	case types.RawResource:
		output = v
	}

	if output != nil {
		return json.NewEncoder(writer).Encode(output)
	}

	return nil
}
func (j *JSONResponseWriter) writeMapSlice(builder *builder.Builder, apiContext *types.APIContext, input []map[string]interface{}) *types.GenericCollection {
	collection := newCollection(apiContext)
	for _, value := range input {
		converted := j.convert(builder, apiContext, value)
		if converted != nil {
			collection.Data = append(collection.Data, converted)
		}
	}

	if apiContext.Schema.CollectionFormatter != nil {
		apiContext.Schema.CollectionFormatter(apiContext, collection)
	}

	return collection
}

func (j *JSONResponseWriter) writeInterfaceSlice(builder *builder.Builder, apiContext *types.APIContext, input []interface{}) *types.GenericCollection {
	collection := newCollection(apiContext)
	for _, value := range input {
		switch v := value.(type) {
		case map[string]interface{}:
			converted := j.convert(builder, apiContext, v)
			if converted != nil {
				collection.Data = append(collection.Data, converted)
			}
		default:
			collection.Data = append(collection.Data, v)
		}
	}

	if apiContext.Schema.CollectionFormatter != nil {
		apiContext.Schema.CollectionFormatter(apiContext, collection)
	}

	return collection
}

func toString(val interface{}) string {
	if val == nil {
		return ""
	}
	return fmt.Sprint(val)
}

func (j *JSONResponseWriter) convert(b *builder.Builder, context *types.APIContext, input map[string]interface{}) *types.RawResource {
	schema := context.Schemas.Schema(context.Version, definition.GetFullType(input))
	if schema == nil {
		return nil
	}
	op := builder.List
	if context.Method == http.MethodPost {
		op = builder.ListForCreate
	}
	data, err := b.Construct(schema, input, op)
	if err != nil {
		logrus.Errorf("Failed to construct object on output: %v", err)
		return nil
	}

	rawResource := &types.RawResource{
		ID:          toString(input["id"]),
		Type:        schema.ID,
		Schema:      schema,
		Links:       map[string]string{},
		Actions:     map[string]string{},
		Values:      data,
		ActionLinks: context.Request.Header.Get("X-API-Action-Links") != "",
	}

	j.addLinks(b, schema, context, input, rawResource)

	if schema.Formatter != nil {
		schema.Formatter(context, rawResource)
	}

	return rawResource
}

func (j *JSONResponseWriter) addLinks(b *builder.Builder, schema *types.Schema, context *types.APIContext, input map[string]interface{}, rawResource *types.RawResource) {
	if rawResource.ID == "" {
		return
	}

	self := context.URLBuilder.ResourceLink(rawResource)
	rawResource.Links["self"] = self
	if context.AccessControl.CanUpdate(context, input, schema) {
		rawResource.Links["update"] = self
	}
	if context.AccessControl.CanDelete(context, input, schema) {
		rawResource.Links["remove"] = self
	}

	for _, backRef := range context.Schemas.References(schema) {
		if !backRef.Schema.CanList(context) {
			continue
		}

		if schema.SubContext == "" {
			rawResource.Links[backRef.Schema.PluralName] = context.URLBuilder.FilterLink(backRef.Schema, backRef.FieldName, rawResource.ID)
		} else {
			rawResource.Links[backRef.Schema.PluralName] = context.URLBuilder.SubContextCollection(schema, rawResource.ID, backRef.Schema)
		}
	}
}

func newCollection(apiContext *types.APIContext) *types.GenericCollection {
	result := &types.GenericCollection{
		Collection: types.Collection{
			Type:         "collection",
			ResourceType: apiContext.Type,
			CreateTypes:  map[string]string{},
			Links: map[string]string{
				"self": apiContext.URLBuilder.Current(),
			},
			Actions: map[string]string{},
		},
		Data: []interface{}{},
	}

	if apiContext.Method == http.MethodGet {
		if apiContext.AccessControl.CanCreate(apiContext, apiContext.Schema) {
			result.CreateTypes[apiContext.Schema.ID] = apiContext.URLBuilder.Collection(apiContext.Schema, apiContext.Version)
		}
	}

	opts := parse.QueryOptions(apiContext, apiContext.Schema)
	result.Sort = &opts.Sort
	result.Sort.Reverse = apiContext.URLBuilder.ReverseSort(result.Sort.Order)
	result.Sort.Links = map[string]string{}
	result.Pagination = opts.Pagination
	result.Filters = map[string][]types.Condition{}

	for _, cond := range opts.Conditions {
		filters := result.Filters[cond.Field]
		result.Filters[cond.Field] = append(filters, cond.ToCondition())
	}

	for name := range apiContext.Schema.CollectionFilters {
		if _, ok := result.Filters[name]; !ok {
			result.Filters[name] = nil
		}
	}

	for queryField := range apiContext.Schema.CollectionFilters {
		field, ok := apiContext.Schema.ResourceFields[queryField]
		if ok && (field.Type == "string" || field.Type == "enum") {
			result.Sort.Links[queryField] = apiContext.URLBuilder.Sort(queryField)
		}
	}

	if result.Pagination.Partial {
		if result.Pagination.Next != "" {
			result.Pagination.Next = apiContext.URLBuilder.Marker(result.Pagination.Next)
		}
		if result.Pagination.Previous != "" {
			result.Pagination.Previous = apiContext.URLBuilder.Marker(result.Pagination.Previous)
		}
		if result.Pagination.First != "" {
			result.Pagination.First = apiContext.URLBuilder.Marker(result.Pagination.First)
		}
		if result.Pagination.Last != "" {
			result.Pagination.Last = apiContext.URLBuilder.Marker(result.Pagination.Last)
		}
	}

	return result
}
