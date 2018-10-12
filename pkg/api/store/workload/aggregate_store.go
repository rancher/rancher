package workload

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/client/project/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
)

const (
	SelectorLabel = "workload.user.cattle.io/workloadselector"
)

type AggregateStore struct {
	Stores          map[string]types.Store
	Schemas         map[string]*types.Schema
	FieldToSchemaID map[string]string
}

func NewAggregateStore(schemas ...*types.Schema) *AggregateStore {
	a := &AggregateStore{
		Stores:          map[string]types.Store{},
		Schemas:         map[string]*types.Schema{},
		FieldToSchemaID: map[string]string{},
	}

	for _, schema := range schemas {
		a.Schemas[strings.ToLower(schema.ID)] = schema
		a.Stores[strings.ToLower(schema.ID)] = schema.Store
		fieldKey := fmt.Sprintf("%sConfig", schema.ID)
		a.FieldToSchemaID[fieldKey] = strings.ToLower(schema.ID)
	}

	return a
}

func (a *AggregateStore) Context() types.StorageContext {
	return config.UserStorageContext
}

func (a *AggregateStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	store, schemaType, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	_, shortID := splitTypeAndID(id)
	data, err := store.ByID(apiContext, a.Schemas[schemaType], shortID)
	if err != nil {
		return nil, err
	}
	return capabilitiesToUpperCase(data), nil
}

func (a *AggregateStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	readerGroup, ctx := errgroup.WithContext(apiContext.Request.Context())
	apiContext.Request = apiContext.Request.WithContext(ctx)

	events := make(chan map[string]interface{})
	for _, schema := range a.Schemas {
		streamStore(readerGroup, apiContext, schema, opt, events)
	}

	go func() {
		readerGroup.Wait()
		close(events)
	}()
	return events, nil
}

func (a *AggregateStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	items := make(chan map[string]interface{})
	g, ctx := errgroup.WithContext(apiContext.Request.Context())
	submit := func(schema *types.Schema, store types.Store) {
		g.Go(func() error {
			data, err := store.List(apiContext, schema, opt)
			if err != nil {
				return err
			}
			for _, item := range data {
				select {
				case items <- capabilitiesToUpperCase(item):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	for typeName, store := range a.Stores {
		submit(a.Schemas[typeName], store)
	}

	go func() {
		g.Wait()
		close(items)
	}()

	var result []map[string]interface{}
	for item := range items {
		result = append(result, item)
	}

	return result, g.Wait()
}

func (a *AggregateStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	// deployment is default if otherwise is not specified
	kind := client.DeploymentType
	toSchema := a.Schemas[kind]
	toStore := a.Stores[kind]
	for field, schemaID := range a.FieldToSchemaID {
		if val, ok := data[field]; ok && val != nil {
			toSchema = a.Schemas[schemaID]
			toStore = a.Stores[schemaID]
			break
		}
	}

	return toStore.Create(apiContext, toSchema, data)
}

func store(registries map[string]projectclient.RegistryCredential, domainToCreds map[string][]corev1.LocalObjectReference, name string) {
	for registry := range registries {
		secretRef := corev1.LocalObjectReference{Name: name}
		if _, ok := domainToCreds[registry]; ok {
			domainToCreds[registry] = append(domainToCreds[registry], secretRef)
		} else {
			domainToCreds[registry] = []corev1.LocalObjectReference{secretRef}
		}
	}
}

func resolveWorkloadID(schemaID string, data map[string]interface{}) string {
	return fmt.Sprintf("%s-%s-%s", schemaID, data["namespaceId"], data["name"])
}

func (a *AggregateStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	store, schemaType, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	_, shortID := splitTypeAndID(id)
	return store.Update(apiContext, a.Schemas[schemaType], data, shortID)
}

func (a *AggregateStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	store, schemaType, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	_, shortID := splitTypeAndID(id)
	return store.Delete(apiContext, a.Schemas[schemaType], shortID)
}

func (a *AggregateStore) getStore(id string) (types.Store, string, error) {
	typeName, _ := splitTypeAndID(id)
	store, ok := a.Stores[typeName]
	if !ok {
		return nil, "", httperror.NewAPIError(httperror.NotFound, "failed to find type "+typeName)
	}
	return store, typeName, nil
}

func streamStore(eg *errgroup.Group, apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions, result chan map[string]interface{}) {
	eg.Go(func() error {
		events, err := schema.Store.Watch(apiContext, schema, opt)
		if err != nil || events == nil {
			if err != nil {
				logrus.Errorf("failed on subscribe %s: %v", schema.ID, err)
			}
			return err
		}

		logrus.Debugf("watching %s", schema.ID)

		for e := range events {
			result <- capabilitiesToUpperCase(e)
		}

		return errors.New("disconnect")
	})
}

func splitTypeAndID(id string) (string, string) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) < 2 {
		// Must conform
		return "", ""
	}
	return parts[0], parts[1]
}

func getKey(key string) string {
	return base64.URLEncoding.EncodeToString([]byte(key))
}

//Related issue: #12619
//In Rancher API schema, Capabilities is defined as enum type and `ALL` is one of the options, which means Rancher API only accepts `ALL`.
//However, Kubernetes accepts both `all` and `ALL`` for capabilities, if user uses kubectl and use `all`` in the yaml, edit will fail in Rancher UI.
//Thus we should convert `all`` to `ALL` so that UI always get `ALL`.
func capabilitiesToUpperCase(data map[string]interface{}) map[string]interface{} {
	containers := convert.ToMapSlice(data["containers"])
	elements := []string{"capDrop", "capAdd"}

	for _, c := range containers {
		for _, element := range elements {
			caps := convert.ToStringSlice(c[element])
			newCaps := []string{}
			if caps != nil {
				for _, cap := range caps {
					newCaps = append(newCaps, strings.ToUpper(cap))
				}
				c[element] = newCaps
			}
		}
	}

	return data
}
