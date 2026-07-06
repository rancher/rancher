package cluster

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
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

	return r.populateCommands(data), nil
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
		items[i] = r.populateCommands(items[i])
	}

	return items, nil
}

func (r *RegistrationTokenStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]any, error) {
	events, err := r.Store.Watch(apiContext, schema, opt)
	if err != nil || events == nil {
		return events, err
	}

	if r.SecretCache == nil {
		return events, nil
	}

	out := make(chan map[string]interface{})
	go func() {
		defer close(out)
		for data := range events {
			data = r.populateCommands(data)
			out <- data
		}
	}()

	return out, nil
}

var commandFields = []string{
	"command",
	"insecureCommand",
	"manifestUrl",
	"nodeCommand",
	"insecureNodeCommand",
	"windowsNodeCommand",
	"insecureWindowsNodeCommand",
}

func (r *RegistrationTokenStore) populateCommands(data map[string]interface{}) map[string]interface{} {
	tokenSecretName := convert.ToString(data["tokenSecretName"])
	logrus.Tracef("[CRT norman] populateCommands: tokenSecretName=%s clusterId=%v", tokenSecretName, data["clusterId"])
	if tokenSecretName == "" {
		return r.clearCommands(data)
	}

	ns := convert.ToString(data["clusterId"])
	if ns == "" {
		return r.clearCommands(data)
	}

	secret, err := r.SecretCache.Get(ns, tokenSecretName)
	if err != nil {
		logrus.Tracef("[CRT norman] error getting secret %s, clearing commands", tokenSecretName)
		return r.clearCommands(data)
	}

	token := string(secret.Data["token"])
	if token == "" {
		logrus.Tracef("[CRT norman] empty token in secret %s, clearing commands", tokenSecretName)
		return r.clearCommands(data)
	}

	data["token"] = token

	for _, field := range commandFields {
		r.replaceTokenPlaceholder(data, field, token)
	}

	return data
}

func (r *RegistrationTokenStore) replaceTokenPlaceholder(data map[string]interface{}, field, token string) {
	if val, ok := data[field].(string); ok && val != "" {
		data[field] = strings.ReplaceAll(val, "{token}", token)
	}
}

func (r *RegistrationTokenStore) clearCommands(data map[string]interface{}) map[string]interface{} {
	for _, field := range commandFields {
		if _, ok := data[field]; ok {
			data[field] = ""
		}
	}
	return data
}
