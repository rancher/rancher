package httpproxy

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/kv"
)

func getAuthData(auth string, secrets SecretGetter, fields []string) (map[string]string, map[string]string, error) {
	params := getRequestParams(auth)
	if !requiredFieldsExist(params, fields) {
		return params, nil, fmt.Errorf("required fields %s not set", fields)
	}
	if params["credID"] == "" {
		return params, nil, nil
	}
	secret, err := getCredential(params["credID"], secrets)
	if err != nil {
		return params, nil, err
	}
	return params, secret, nil
}

func getRequestParams(auth string) map[string]string {
	params := map[string]string{}
	if auth == "" {
		return params
	}

	terms := strings.Fields(auth)
	for _, term := range terms[1:] {
		splitTerm := strings.SplitN(term, "=", 2)
		if len(splitTerm) != 2 || splitTerm[0] == "" {
			continue
		}
		params[splitTerm[0]] = splitTerm[1]
	}
	return params
}

// credentialIDFromCattleAuth extracts credID from X-API-CattleAuth-Header.
// It supports both parameterized values ("<mode> credID=<ns>/<name>") and
// bare credential IDs ("<ns>/<name>" or "provider:<name>").
func credentialIDFromCattleAuth(cAuth string) string {
	params := getRequestParams(cAuth)
	if credID := params["credID"]; credID != "" {
		return credID
	}

	bare := strings.TrimSpace(cAuth)
	if bare == "" || strings.Contains(bare, " ") || strings.Contains(bare, "=") {
		return ""
	}
	if strings.Contains(bare, "/") || strings.Contains(bare, ":") {
		return bare
	}
	return ""
}

func requiredFieldsExist(data map[string]string, fields []string) bool {
	for _, field := range fields {
		if val, ok := data[field]; !ok || val == "" {
			return false
		}
	}
	return true
}

func getCredential(credentialID string, credentials SecretGetter) (map[string]string, error) {
	ns, name := kv.Split(credentialID, "/")
	if name == "" {
		split := strings.SplitN(credentialID, ":", 2)
		if len(split) != 2 || split[0] == "" || split[1] == "" {
			return nil, fmt.Errorf("invalid credential id %s", credentialID)
		}
		ns = namespace.GlobalNamespace
		name = split[1]
	}
	cred, err := credentials(ns, name)
	if err != nil {
		return nil, err
	}
	ans := map[string]string{}
	for key, val := range cred.Data {
		splitKeys := strings.Split(key, "-")
		if len(splitKeys) == 2 && strings.HasSuffix(splitKeys[0], "Config") {
			ans[splitKeys[1]] = string(val)
		} else {
			ans[key] = string(val)
		}
	}
	return ans, nil
}
