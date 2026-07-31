package transform

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type TransformerFunc func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error)

type ListTransformerFunc func(apiContext *types.APIContext, schema *types.Schema, data []map[string]interface{}, opt *types.QueryOptions) ([]map[string]interface{}, error)

type StreamTransformerFunc func(apiContext *types.APIContext, schema *types.Schema, data chan map[string]interface{}, opt *types.QueryOptions) (chan map[string]interface{}, error)

type Store struct {
	Store             types.Store
	Transformer       TransformerFunc
	ListTransformer   ListTransformerFunc
	StreamTransformer StreamTransformerFunc
}

func (s *Store) Context() types.StorageContext {
	return s.Store.Context()
}

func (s *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := s.Store.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	if s.Transformer == nil {
		return data, nil
	}
	obj, err := s.Transformer(apiContext, schema, data, &types.QueryOptions{
		Options: map[string]string{
			"ByID": "true",
		},
	})
	if obj == nil && err == nil {
		return obj, httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("%s not found", id))
	}
	return obj, err
}

func (s *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	c, err := s.Store.Watch(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}

	if s.StreamTransformer != nil {
		return s.StreamTransformer(apiContext, schema, c, opt)
	}

	return convert.Chan(c, func(data map[string]interface{}) map[string]interface{} {
		item, err := s.Transformer(apiContext, schema, data, opt)
		if err != nil {
			return nil
		}
		return item
	}), nil
}

func (s *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	data, err := s.Store.List(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}

	if s.ListTransformer != nil {
		return s.ListTransformer(apiContext, schema, data, opt)
	}

	if s.Transformer == nil {
		return data, nil
	}

	var result []map[string]interface{}
	for _, item := range data {
		item, err := s.Transformer(apiContext, schema, item, opt)
		if err != nil {
			return nil, err
		}
		if item != nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	data, err := s.Store.Create(apiContext, schema, data)
	if err != nil {
		return nil, err
	}
	if s.Transformer == nil {
		return data, nil
	}
	return s.Transformer(apiContext, schema, data, nil)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	data, err := s.Store.Update(apiContext, schema, data, id)
	if err != nil {
		return nil, err
	}
	if s.Transformer == nil {
		return data, nil
	}
	return s.Transformer(apiContext, schema, data, nil)
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	obj, err := s.Store.Delete(apiContext, schema, id)
	if err != nil || obj == nil {
		return obj, err
	}
	return s.Transformer(apiContext, schema, obj, nil)
}
