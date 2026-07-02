package cluster

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

type RegistrationTokenStore struct {
	types.Store
	SecretCache corecontrollers.SecretCache
}

func (r *RegistrationTokenStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := r.Store.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

	if r.SecretCache == nil {
		return data, nil
	}

	return r.populateCommands(data)
}

func (r *RegistrationTokenStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	items, err := r.Store.List(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}

	if r.SecretCache == nil {
		return items, nil
	}

	// Populate commands for each CRT in the list
	for i := range items {
		items[i], _ = r.populateCommands(items[i])
	}

	return items, nil
}

func (r *RegistrationTokenStore) populateCommands(data map[string]interface{}) (map[string]interface{}, error) {
	tokenSecretName := convert.ToString(data["tokenSecretName"])
	if tokenSecretName == "" {
		return data, nil
	}

	ns := convert.ToString(data["clusterId"])
	if ns == "" {
		return data, nil
	}

	secret, err := r.SecretCache.Get(ns, tokenSecretName)
	if err != nil {
		return data, nil
	}

	token := string(secret.Data["token"])
	if token == "" {
		return data, nil
	}

	data["token"] = token

	r.replaceTokenPlaceholder(data, "command", token)
	r.replaceTokenPlaceholder(data, "insecureCommand", token)
	r.replaceTokenPlaceholder(data, "manifestUrl", token)
	r.replaceTokenPlaceholder(data, "nodeCommand", token)
	r.replaceTokenPlaceholder(data, "insecureNodeCommand", token)
	r.replaceTokenPlaceholder(data, "windowsNodeCommand", token)
	r.replaceTokenPlaceholder(data, "insecureWindowsNodeCommand", token)

	return data, nil
}

func (r *RegistrationTokenStore) replaceTokenPlaceholder(data map[string]interface{}, field, token string) {
	if val, ok := data[field].(string); ok && val != "" {
		data[field] = strings.ReplaceAll(val, "{token}", token)
	}
}
