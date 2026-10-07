package common

import "strings"

func TransformToAuthProvider(authConfig map[string]any) map[string]any {
	result := map[string]any{}

	metadata, ok := authConfig["metadata"].(map[string]any)
	if ok {
		if name, found := metadata["name"]; found {
			result["id"] = name
		}
	}

	if t, _ := authConfig["type"].(string); t != "" {
		p := strings.ReplaceAll(t, "Config", "Provider")
		// The config type "googleOauthConfig" produces "googleOauthProvider" but
		// the canonical provider type is "googleOAuthProvider" (capital 'A').
		if p == "googleOauthProvider" {
			p = "googleOAuthProvider"
		}
		result["type"] = p
	}

	for _, key := range []string{"logoutAllSupported", "logoutAllEnabled", "logoutAllForced"} {
		value, _ := authConfig[key].(bool)
		result[key] = value
	}

	return result
}
