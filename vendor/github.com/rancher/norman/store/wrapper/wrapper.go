package wrapper

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

func Wrap(store types.Store) types.Store {
	if _, ok := store.(*StoreWrapper); ok {
		return store
	}

	return &StoreWrapper{
		store,
	}
}

type StoreWrapper struct {
	types.Store
}

func (s *StoreWrapper) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := s.Store.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

	return apiContext.FilterObject(&types.QueryOptions{
		Conditions: apiContext.SubContextAttributeProvider.Query(apiContext, schema),
	}, schema, data), nil
}

func (s *StoreWrapper) List(apiContext *types.APIContext, schema *types.Schema, opts *types.QueryOptions) ([]map[string]interface{}, error) {
	opts.Conditions = append(opts.Conditions, apiContext.SubContextAttributeProvider.Query(apiContext, schema)...)
	data, err := s.Store.List(apiContext, schema, opts)
	if err != nil {
		return nil, err
	}

	return apiContext.FilterList(opts, schema, data), nil
}

func (s *StoreWrapper) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	c, err := s.Store.Watch(apiContext, schema, opt)
	if err != nil || c == nil {
		return nil, err
	}

	return convert.Chan(c, func(data map[string]interface{}) map[string]interface{} {
		return apiContext.FilterObject(&types.QueryOptions{
			Conditions: apiContext.SubContextAttributeProvider.Query(apiContext, schema),
		}, schema, data)
	}), nil
}

func (s *StoreWrapper) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	for key, value := range apiContext.SubContextAttributeProvider.Create(apiContext, schema) {
		if data == nil {
			data = map[string]interface{}{}
		}
		data[key] = value
	}

	data, err := s.Store.Create(apiContext, schema, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *StoreWrapper) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	err := validateGet(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

	data, err = s.Store.Update(apiContext, schema, data, id)
	if err != nil {
		return nil, err
	}

	return apiContext.FilterObject(&types.QueryOptions{
		Conditions: apiContext.SubContextAttributeProvider.Query(apiContext, schema),
	}, schema, data), nil
}

func (s *StoreWrapper) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	if err := validateGet(apiContext, schema, id); err != nil {
		return nil, err
	}

	return s.Store.Delete(apiContext, schema, id)
}

func validateGet(apiContext *types.APIContext, schema *types.Schema, id string) error {
	Store := schema.Store
	if Store == nil {
		return nil
	}

	existing, err := Store.ByID(apiContext, schema, id)
	if err != nil {
		return err
	}

	if apiContext.Filter(&types.QueryOptions{
		Conditions: apiContext.SubContextAttributeProvider.Query(apiContext, schema),
	}, schema, existing) == nil {
		return httperror.NewAPIError(httperror.NotFound, "failed to find "+id)
	}

	return nil
}
