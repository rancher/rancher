package merge

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

func APIUpdateMerge(schema *types.Schema, schemas *types.Schemas, dest, src map[string]interface{}, replace bool) map[string]interface{} {
	result := map[string]interface{}{}
	if replace {
		if status, ok := dest["status"]; ok {
			result["status"] = status
		}
		if metadata, ok := dest["metadata"]; ok {
			result["metadata"] = metadata
		}
	} else {
		result = copyMap(dest)
	}

	for k, v := range src {
		if k == "metadata" {
			result["metadata"] = mergeMetadata(convert.ToMapInterface(dest["metadata"]), convert.ToMapInterface(v))
			continue
		} else if k == "status" {
			continue
		}

		existing, ok := dest[k]
		if ok && !replace {
			result[k] = merge(k, schema, schemas, existing, v)
		} else {
			result[k] = v
		}
	}

	return result
}

func isProtected(k string) bool {
	if !strings.Contains(k, "cattle.io/") || strings.HasPrefix(k, "field.cattle.io/") {
		return false
	}
	return true
}

func mergeProtected(dest, src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return dest
	}

	result := copyMap(dest)

	for k, v := range src {
		if isProtected(k) {
			continue
		}
		result[k] = v
	}

	for k := range dest {
		if isProtected(k) {
			continue
		}
		if _, ok := src[k]; !ok {
			delete(dest, k)
		}
	}

	return result
}

func mergeMetadata(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	result := copyMap(dest)

	labels := convert.ToMapInterface(dest["labels"])
	srcLabels := convert.ToMapInterface(src["labels"])
	labels = mergeProtected(labels, srcLabels)

	annotations := convert.ToMapInterface(dest["annotations"])
	srcAnnotation := convert.ToMapInterface(src["annotations"])
	annotations = mergeProtected(annotations, srcAnnotation)

	result["labels"] = labels
	result["annotations"] = annotations

	return result
}

func merge(field string, schema *types.Schema, schemas *types.Schemas, dest, src interface{}) interface{} {
	if isMap(field, schema) {
		return src
	}

	sm, smOk := src.(map[string]interface{})
	dm, dmOk := dest.(map[string]interface{})
	if smOk && dmOk {
		return mergeMaps(getSchema(field, schema, schemas), schemas, dm, sm)
	}
	return src
}

func getSchema(field string, schema *types.Schema, schemas *types.Schemas) *types.Schema {
	if schema == nil {
		return nil
	}
	s := schemas.Schema(&schema.Version, schema.ResourceFields[field].Type)
	if s != nil && s.InternalSchema != nil {
		return s.InternalSchema
	}
	return s
}

func isMap(field string, schema *types.Schema) bool {
	if schema == nil {
		return false
	}
	f := schema.ResourceFields[field]
	return strings.HasPrefix(f.Type, "map[")
}

func mergeMaps(schema *types.Schema, schemas *types.Schemas, dest map[string]interface{}, src map[string]interface{}) interface{} {
	result := copyMap(dest)
	for k, v := range src {
		result[k] = merge(k, schema, schemas, dest[k], v)
	}
	return result
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range src {
		result[k] = v
	}
	return result
}
