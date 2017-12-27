package workload

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"golang.org/x/sync/errgroup"
)

type AggregateStore struct {
	CreateStore types.Store
	Stores      map[string]types.Store
	Schemas     map[string]*types.Schema
}

func NewAggregateStore(createStore types.Store, schemas ...*types.Schema) *AggregateStore {
	a := &AggregateStore{
		CreateStore: createStore,
		Stores:      map[string]types.Store{},
		Schemas:     map[string]*types.Schema{},
	}

	for _, schema := range schemas {
		a.Schemas[strings.ToLower(schema.ID)] = schema
		a.Stores[strings.ToLower(schema.ID)] = schema.Store
	}

	return a
}

func (a *AggregateStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	store, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	return store.ByID(apiContext, schema, id)
}

func (a *AggregateStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return a.CreateStore.Watch(apiContext, schema, opt)
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
				case items <- item:
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
	return a.CreateStore.Create(apiContext, schema, data)
}

func (a *AggregateStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	store, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	return store.Update(apiContext, schema, data, id)
}

func (a *AggregateStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	store, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	return store.Delete(apiContext, schema, id)
}

func (a *AggregateStore) getStore(id string) (types.Store, error) {
	typeName, _ := splitTypeAndID(id)
	store, ok := a.Stores[typeName]
	if !ok {
		return nil, httperror.NewAPIError(httperror.NotFound, "failed to find type "+typeName)
	}
	return store, nil
}
