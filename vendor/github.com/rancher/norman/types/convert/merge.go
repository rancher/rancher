package convert

import (
	"strings"
)

func APIUpdateMerge(dest, src map[string]interface{}, replace bool) map[string]interface{} {
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
			result["metadata"] = mergeMetadata(ToMapInterface(dest["metadata"]), ToMapInterface(v))
		} else if k == "status" {
			continue
		}

		existing, ok := dest[k]
		if ok && !replace {
			result[k] = merge(existing, v)
		} else {
			result[k] = v
		}
	}

	return result
}

func mergeMetadata(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	result := copyMap(dest)

	labels := mergeMaps(ToMapInterface(dest["labels"]), ToMapInterface(src["labels"]))

	existingAnnotation := ToMapInterface(dest["annotations"])
	newAnnotation := ToMapInterface(src["annotations"])
	annotations := copyMap(existingAnnotation)

	for k, v := range newAnnotation {
		if strings.Contains(k, "cattle.io/") {
			continue
		}
		annotations[k] = v
	}
	for k, v := range existingAnnotation {
		if strings.Contains(k, "cattle.io/") {
			annotations[k] = v
		}
	}

	result["labels"] = labels
	result["annotations"] = annotations

	return result
}

func merge(dest, src interface{}) interface{} {
	sm, smOk := src.(map[string]interface{})
	dm, dmOk := dest.(map[string]interface{})
	if smOk && dmOk {
		return mergeMaps(dm, sm)
	}
	return src
}

func mergeMaps(dest map[string]interface{}, src map[string]interface{}) interface{} {
	result := copyMap(dest)
	for k, v := range src {
		result[k] = merge(dest[k], v)
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
