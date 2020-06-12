package apiroot

import (
	"net/http"
	"path"
	"strings"

	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas"
)

func Register(apiSchemas *types.APISchemas, versions []string, roots ...string) {
	apiSchemas.MustAddSchema(types.APISchema{
		Schema: &schemas.Schema{
			ID:                "apiRoot",
			CollectionMethods: []string{"GET"},
			ResourceMethods:   []string{"GET"},
			ResourceFields: map[string]schemas.Field{
				"apiVersion": {Type: "map[json]"},
				"path":       {Type: "string"},
			},
		},
		Formatter: Formatter,
		Store:     NewAPIRootStore(versions, roots),
	})
}

func Formatter(apiOp *types.APIRequest, resource *types.RawResource) {
	data := resource.APIObject.Data()
	path, _ := data["path"].(string)
	if path == "" {
		return
	}
	delete(data, "path")

	resource.Links["root"] = apiOp.URLBuilder.RelativeToRoot(path)

	if data, isAPIRoot := data["apiVersion"].(map[string]interface{}); isAPIRoot {
		apiVersion := apiVersionFromMap(apiOp.Schemas, data)
		for _, schema := range apiOp.Schemas.Schemas {
			addCollectionLink(apiOp, schema, apiVersion, resource.Links)
		}
		resource.Links["self"] = apiOp.URLBuilder.RelativeToRoot(apiVersion)
		resource.Links["schemas"] = apiOp.URLBuilder.RelativeToRoot(path)
	}

	return
}

func addCollectionLink(apiOp *types.APIRequest, schema *types.APISchema, apiVersion string, links map[string]string) {
	collectionLink := getSchemaCollectionLink(apiOp, schema)
	if collectionLink != "" {
		links[schema.PluralName] = apiOp.URLBuilder.RelativeToRoot(path.Join(apiVersion, path.Base(collectionLink)))
	}
}

func getSchemaCollectionLink(apiOp *types.APIRequest, schema *types.APISchema) string {
	if schema != nil && contains(schema.CollectionMethods, http.MethodGet) {
		return apiOp.URLBuilder.Collection(schema)
	}
	return ""
}

type Store struct {
	empty.Store
	roots    []string
	versions []string
}

func NewAPIRootStore(versions []string, roots []string) types.Store {
	return &Store{
		roots:    roots,
		versions: versions,
	}
}

func (a *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return types.DefaultByID(a, apiOp, schema, id)
}

func (a *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	var roots types.APIObjectList

	versions := a.versions

	for _, version := range versions {
		roots.Objects = append(roots.Objects, types.APIObject{
			Type:   "apiRoot",
			ID:     version,
			Object: apiVersionToAPIRootMap(version),
		})
	}

	for _, root := range a.roots {
		parts := strings.SplitN(root, ":", 2)
		if len(parts) == 2 {
			roots.Objects = append(roots.Objects, types.APIObject{
				Type: "apiRoot",
				ID:   parts[0],
				Object: map[string]interface{}{
					"id":   parts[0],
					"path": parts[1],
				},
			})
		}
	}

	return roots, nil
}

func apiVersionToAPIRootMap(version string) map[string]interface{} {
	return map[string]interface{}{
		"id":   version,
		"type": "apiRoot",
		"apiVersion": map[string]interface{}{
			"version": version,
		},
		"path": "/" + version,
	}
}

func apiVersionFromMap(schemas *types.APISchemas, apiVersion map[string]interface{}) string {
	version, _ := apiVersion["version"].(string)
	return version
}

func contains(list []string, needle string) bool {
	for _, v := range list {
		if v == needle {
			return true
		}
	}
	return false
}
