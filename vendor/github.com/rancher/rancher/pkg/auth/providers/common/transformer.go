package common

import "strings"

func TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	if m, ok := authConfig["metadata"].(map[string]interface{}); ok {
		result["id"] = m["name"]
	}
	if t, ok := authConfig["type"].(string); ok && t != "" {
		result["type"] = strings.Replace(t, "Config", "Provider", -1)
	}
	return result
}
