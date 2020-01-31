package builtin

import (
	"net/http"

	"github.com/rancher/steve/pkg/schemaserver/store/schema"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/slice"
)

var (
	Schema = types.APISchema{
		Schema: &schemas.Schema{
			ID:                "schema",
			PluralName:        "schemas",
			CollectionMethods: []string{"GET"},
			ResourceMethods:   []string{"GET"},
			ResourceFields: map[string]schemas.Field{
				"collectionActions": {Type: "map[json]"},
				"collectionFields":  {Type: "map[json]"},
				"collectionFilters": {Type: "map[json]"},
				"collectionMethods": {Type: "array[string]"},
				"pluralName":        {Type: "string"},
				"resourceActions":   {Type: "map[json]"},
				"attributes":        {Type: "map[json]"},
				"resourceFields":    {Type: "map[json]"},
				"resourceMethods":   {Type: "array[string]"},
				"version":           {Type: "map[json]"},
			},
		},
		Formatter: SchemaFormatter,
		Store:     schema.NewSchemaStore(),
	}

	Error = types.APISchema{
		Schema: &schemas.Schema{
			ID:                "error",
			ResourceMethods:   []string{},
			CollectionMethods: []string{},
			ResourceFields: map[string]schemas.Field{
				"code":      {Type: "string"},
				"detail":    {Type: "string", Nullable: true},
				"message":   {Type: "string", Nullable: true},
				"fieldName": {Type: "string", Nullable: true},
				"status":    {Type: "int"},
			},
		},
	}

	Collection = types.APISchema{
		Schema: &schemas.Schema{
			ID:                "collection",
			ResourceMethods:   []string{},
			CollectionMethods: []string{},
			ResourceFields: map[string]schemas.Field{
				"data":       {Type: "array[json]"},
				"pagination": {Type: "map[json]"},
				"sort":       {Type: "map[json]"},
				"filters":    {Type: "map[json]"},
			},
		},
	}

	Schemas = types.EmptyAPISchemas().
		MustAddSchema(Schema).
		MustAddSchema(Error).
		MustAddSchema(Collection)
)

func SchemaFormatter(apiOp *types.APIRequest, resource *types.RawResource) {
	schema := apiOp.Schemas.LookupSchema(resource.ID)
	if schema == nil {
		return
	}

	collectionLink := getSchemaCollectionLink(apiOp, schema)
	if collectionLink != "" {
		resource.Links["collection"] = collectionLink
	}
}

func getSchemaCollectionLink(apiOp *types.APIRequest, schema *types.APISchema) string {
	if schema != nil && slice.ContainsString(schema.CollectionMethods, http.MethodGet) {
		return apiOp.URLBuilder.Collection(schema)
	}
	return ""
}
