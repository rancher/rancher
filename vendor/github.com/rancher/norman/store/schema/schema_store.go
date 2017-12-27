package schema

import (
	"encoding/json"

	"strings"

	"github.com/rancher/norman/store/empty"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
)

type Store struct {
	empty.Store
}

func NewSchemaStore() types.Store {
	return &Store{}
}

func (s *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	for _, schema := range apiContext.Schemas.SchemasForVersion(*apiContext.Version) {
		if strings.EqualFold(schema.ID, id) {
			schemaData := map[string]interface{}{}

			data, err := json.Marshal(schema)
			if err != nil {
				return nil, err
			}

			return schemaData, json.Unmarshal(data, &schemaData)
		}
	}
	return nil, nil
}

func (s *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return nil, nil
}

func (s *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	schemaMap := apiContext.Schemas.SchemasForVersion(*apiContext.Version)
	schemas := make([]*types.Schema, 0, len(schemaMap))
	schemaData := make([]map[string]interface{}, 0, len(schemaMap))

	included := map[string]bool{}

	for _, schema := range schemaMap {
		if included[schema.ID] {
			continue
		}

		if schema.CanList() {
			schemas = addSchema(schema, schemaMap, schemas, included)
		}
	}

	data, err := json.Marshal(schemas)
	if err != nil {
		return nil, err
	}

	return schemaData, json.Unmarshal(data, &schemaData)
}

func addSchema(schema *types.Schema, schemaMap map[string]*types.Schema, schemas []*types.Schema, included map[string]bool) []*types.Schema {
	included[schema.ID] = true
	schemas = traverseAndAdd(schema, schemaMap, schemas, included)
	schemas = append(schemas, schema)
	return schemas
}

func traverseAndAdd(schema *types.Schema, schemaMap map[string]*types.Schema, schemas []*types.Schema, included map[string]bool) []*types.Schema {
	for _, field := range schema.ResourceFields {
		t := ""
		subType := field.Type
		for subType != t {
			t = subType
			subType = definition.SubType(t)
		}

		if refSchema, ok := schemaMap[t]; ok && !included[t] {
			schemas = addSchema(refSchema, schemaMap, schemas, included)
		}
	}

	for _, action := range schema.ResourceActions {
		for _, t := range []string{action.Output, action.Input} {
			if t == "" {
				continue
			}

			if refSchema, ok := schemaMap[t]; ok && !included[t] {
				schemas = addSchema(refSchema, schemaMap, schemas, included)
			}
		}
	}

	return schemas
}
