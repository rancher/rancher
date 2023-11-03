package httpproxy

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v2/pkg/kv"
)

func getAuthData(auth string, secrets SecretGetter, fields []string) (map[string]string, map[string]string, error) {
	data := getRequestParams(auth)
	if !requiredFieldsExist(data, fields) {
		return data, nil, fmt.Errorf("required fields %s not set", fields)
	}
	if data["credID"] == "" {
		return data, nil, nil
	}
	secret, err := getCredential(data["credID"], secrets)
	if err != nil {
		return data, nil, err
	}
	return data, secret, nil
}

func getRequestParams(auth string) map[string]string {
	params := map[string]string{}
	if auth != "" {
		splitAuth := strings.Split(auth, " ")
		for _, term := range splitAuth[1:] {
			splitTerm := strings.SplitN(term, "=", 2)
			params[splitTerm[0]] = splitTerm[1]
		}
	}
	return params
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
