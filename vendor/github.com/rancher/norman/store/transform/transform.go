package transform

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/golang-lru"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/sirupsen/logrus"
	"strings"
)

const maxCacheSize = 5000000

var (
	resultCache, _ = lru.New(maxCacheSize)
)

type MemoryCache struct{
	*lru.Cache
}

type key struct {
	Resource string
}

type value struct {
	Val []map[string]interface{}
}
type TransformerFunc func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error)

type ListTransformerFunc func(apiContext *types.APIContext, schema *types.Schema, data []map[string]interface{}, opt *types.QueryOptions) ([]map[string]interface{}, error)

type StreamTransformerFunc func(apiContext *types.APIContext, schema *types.Schema, data chan map[string]interface{}, opt *types.QueryOptions) (chan map[string]interface{}, error)

type Store struct {
	Store             types.Store
	Transformer       TransformerFunc
	ListTransformer   ListTransformerFunc
	StreamTransformer StreamTransformerFunc
}

func (m *MemoryCache) Add(k key, value interface{}) {
	if bytes, err := json.Marshal(value); len(bytes) < maxCacheSize && err == nil {
		for m.Size() + len(bytes) > maxCacheSize {
			logrus.Info("TEST PUSHING OUT OLDEST")
			m.RemoveOldest()
		}
		m.Cache.Add(k, value)
	}
}

func getKey(ApiContext types.APIContext) string {
	url := ApiContext.Request.URL.Path

	if strings.Contains(url, "pod") {
		return "pods"
	}

	if strings.Contains(url, "config") {
		return "configmaps"
	}

	if strings.Contains(url, "workload") {
		return "workloads"
	}

	if strings.Contains(url, "service") {
		return "services"
	}

	return ""
}

func (m *MemoryCache) ResetCacheResource(apiContext types.APIContext) {
	key := key{
		getKey(apiContext),
	}

	memCache := MemoryCache{
		resultCache,
	}

	if key.Resource != "" {
		memCache.Remove(key)
	}
}

func (m *MemoryCache) Size() int{
	size := 0
	for k,_ := range m.Keys() {
		obj, _ := m.Get(k)
		s, _ := json.Marshal(obj)
		size += len(s)
	}

	return size
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
	key := key{
		getKey(*apiContext),
	}

	memCache := MemoryCache{
		resultCache,
	}

	val, ok := memCache.Get(key)
	if ok {
		value, _ := val.(value)
		return value.Val, nil
	}

	data, err := s.Store.List(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}

	if key.Resource != "" {
		memCache.Add(key, value{
			data,
		})
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
	memCache := MemoryCache{
		resultCache,
	}

	memCache.ResetCacheResource(*apiContext)

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
	memCache := MemoryCache{
		resultCache,
	}

	memCache.ResetCacheResource(*apiContext)

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
	memCache := MemoryCache{
		resultCache,
	}

	memCache.ResetCacheResource(*apiContext)

	obj, err := s.Store.Delete(apiContext, schema, id)
	if err != nil || obj == nil {
		return obj, err
	}
	return s.Transformer(apiContext, schema, obj, nil)
}
