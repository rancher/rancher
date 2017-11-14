package builtin

import (
	"github.com/rancher/norman/store/empty"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

func APIRootFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	path, _ := resource.Values["path"].(string)
	if path == "" {
		return
	}

	delete(resource.Values, "path")

	resource.Links["root"] = apiContext.URLBuilder.RelativeToRoot(path)

	data, _ := resource.Values["apiVersion"].(map[string]interface{})
	apiVersion := apiVersionFromMap(apiContext.Schemas, data)

	resource.Links["self"] = apiContext.URLBuilder.Version(apiVersion)

	if len(apiVersion.SubContexts) > 0 {
		subContextToSchema := apiContext.Schemas.SubContextSchemas()
		if len(subContextToSchema) > 0 {
			for _, schema := range subContextToSchema {
				addCollectionLink(apiContext, schema, resource.Links)
			}

			for _, schema := range getNonReferencedSchemas(apiContext.Schemas.SchemasForVersion(apiVersion),
				subContextToSchema) {
				addCollectionLink(apiContext, schema, resource.Links)
			}

			return
		}
	}

	for _, schema := range apiContext.Schemas.SchemasForVersion(apiVersion) {
		addCollectionLink(apiContext, schema, resource.Links)
	}

	return
}

func getNonReferencedSchemas(schemas map[string]*types.Schema, subContexts map[string]*types.Schema) []*types.Schema {
	var result []*types.Schema
	typeNames := map[string]bool{}

	for _, subContext := range subContexts {
		ref := convert.ToReference(subContext.ID)
		fullRef := convert.ToFullReference(subContext.Version.Path, subContext.ID)
		typeNames[ref] = true
		typeNames[fullRef] = true
	}

outer:
	for _, schema := range schemas {
		for _, field := range schema.ResourceFields {
			if typeNames[field.Type] {
				continue outer
			}
		}

		result = append(result, schema)
	}

	return result
}

func addCollectionLink(apiContext *types.APIContext, schema *types.Schema, links map[string]string) {
	collectionLink := getSchemaCollectionLink(apiContext, schema, nil)
	if collectionLink != "" {
		links[schema.PluralName] = collectionLink
	}
}

type APIRootStore struct {
	empty.Store
	roots []string
}

func NewAPIRootStore(roots []string) types.Store {
	return &APIRootStore{roots: roots}
}

func (a *APIRootStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	for _, version := range apiContext.Schemas.Versions() {
		if version.Path == id {
			return apiVersionToAPIRootMap(version), nil
		}
	}
	return nil, nil
}

func (a *APIRootStore) List(apiContext *types.APIContext, schema *types.Schema, opt types.QueryOptions) ([]map[string]interface{}, error) {
	var roots []map[string]interface{}

	for _, version := range apiContext.Schemas.Versions() {
		roots = append(roots, apiVersionToAPIRootMap(version))
	}

	for _, root := range a.roots {
		roots = append(roots, map[string]interface{}{
			"path": root,
		})
	}

	return roots, nil
}

func apiVersionToAPIRootMap(version types.APIVersion) map[string]interface{} {
	return map[string]interface{}{
		"type": "/meta/schemas/apiRoot",
		"apiVersion": map[string]interface{}{
			"version": version.Version,
			"group":   version.Group,
			"path":    version.Path,
		},
		"path": version.Path,
	}
}
