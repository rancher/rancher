package common

import "strings"

func TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}

	metadata, ok := authConfig["metadata"].(map[string]interface{})
	if ok {
		if name, found := metadata["name"]; found {
			result["id"] = name
		}
	}

	if t, _ := authConfig["type"].(string); t != "" {
		result["type"] = strings.Replace(t, "Config", "Provider", -1)
	}

	return result
}
