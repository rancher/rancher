package httpproxy

import (
	"fmt"
	"strings"

	v1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/namespace"
)

func getAuthData(auth string, secrets v1.SecretInterface, fields []string) (map[string]string, map[string]string, error) {
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

func getCredential(credentialID string, credentials v1.SecretInterface) (map[string]string, error) {
	split := strings.SplitN(credentialID, ":", 2)
	if len(split) != 2 || split[0] == "" || split[1] == "" {
		return nil, fmt.Errorf("invalid credential id %s", credentialID)
	}
	cred, err := credentials.Controller().Lister().Get(namespace.GlobalNamespace, split[1])
	if err != nil {
		return nil, err
	}
	ans := map[string]string{}
	for key, val := range cred.Data {
		splitKeys := strings.Split(key, "-")
		if len(splitKeys) == 2 && strings.HasSuffix(splitKeys[0], "Config") {
			ans[splitKeys[1]] = string(val)
		}
	}
	return ans, nil
}
