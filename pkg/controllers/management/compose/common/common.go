package common

import (
	"strings"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
)

func SortSchema(schemas map[string]types.Schema) []string {
	inserted := map[string]bool{}
	result := []string{}
	for i := 0; i < 100; i++ {
		for k, schema := range schemas {
			if inserted[k] {
				continue
			}
			ready := true
			for fieldName, field := range schema.ResourceFields {
				if strings.Contains(field.Type, "reference") {
					reference := GetReference(field.Type)
					if isNamespaceIDRef(reference, k) {
						continue
					}
					if !inserted[reference] && fieldName != "creatorId" && k != reference {
						ready = false
					}
				}
			}
			if ready {
				inserted[k] = true
				result = append(result, k)
			}
		}
	}
	return result
}

var (
	namespacedSchema = map[string]bool{
		"project": true,
	}
)

func isNamespaceIDRef(ref, schemaType string) bool {
	if ref == "namespace" && namespacedSchema[schemaType] {
		return true
	}
	return false
}

func GetReference(name string) string {
	name = strings.TrimSuffix(strings.TrimPrefix(name, "array["), "]")
	r := strings.TrimSuffix(strings.TrimPrefix(name, "reference["), "]")
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(r, "/v3/schemas/"), "/v3/clusters/schemas/"), "/v3/projects/schemas/")
}

// ReplaceGlobalReference replace name to id
func ReplaceGlobalReference(schema types.Schema, data map[string]interface{}, referenceMap map[string]map[string]string, client *clientbase.APIBaseClient) error {
	for key, field := range schema.ResourceFields {
		if strings.Contains(field.Type, "reference") {
			reference := GetReference(field.Type)
			if _, ok := data[key]; !ok {
				continue
			}
			if err := FillInReferenceMap(client, reference, referenceMap, nil); err != nil {
				return err
			}
			if strings.HasPrefix(field.Type, "array") {
				r := []string{}
				for _, k := range data[key].([]interface{}) {
					r = append(r, referenceMap[reference][k.(string)])
				}
				data[key] = r
			} else {
				if referenceMap[reference][data[key].(string)] != "" {
					data[key] = referenceMap[reference][data[key].(string)]
				}
			}
		}
	}
	return nil
}

func FillInReferenceMap(client *clientbase.APIBaseClient, schemaKey string, referenceMap map[string]map[string]string, filter map[string]string) error {
	if _, ok := referenceMap[schemaKey]; ok {
		return nil
	}
	referenceMap[schemaKey] = map[string]string{}
	respObj := map[string]interface{}{}
	if err := client.List(schemaKey, &types.ListOpts{}, &respObj); err != nil {
		return err
	}
	if data, ok := respObj["data"]; ok {
		if collections, ok := data.([]interface{}); ok {
			for _, obj := range collections {
				if objMap, ok := obj.(map[string]interface{}); ok {
					id := GetValue(objMap, "id")
					name := GetValue(objMap, "name")
					filtered := true
					for k, v := range filter {
						if GetValue(objMap, k) != v {
							filtered = false
						}
					}
					if filtered {
						referenceMap[schemaKey][name] = id
					}
				}
			}
		}
	}
	return nil
}

func GetValue(data map[string]interface{}, key string) string {
	if v, ok := data[key]; ok {
		if _, ok := v.(string); ok {
			return v.(string)
		}
	}
	return ""
}
