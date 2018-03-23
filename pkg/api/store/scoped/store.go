package scoped

import (
	"strings"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/client/management/v3"
)

type Store struct {
	types.Store
	key string
}

func NewScopedStore(key string, store types.Store) *Store {
	return &Store{
		Store: &transform.Store{
			Store: store,
			Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
				if data == nil {
					return data, nil
				}
				v := convert.ToString(data[key])
				if !strings.HasSuffix(v, ":"+convert.ToString(data[client.ProjectFieldNamespaceId])) {
					data[key] = data[client.ProjectFieldNamespaceId]
				}
				data[client.ProjectFieldNamespaceId] = nil
				return data, nil
			},
		},
		key: key,
	}
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if data != nil {
		parts := strings.Split(convert.ToString(data[s.key]), ":")
		data["namespaceId"] = parts[len(parts)-1]
	}

	return s.Store.Create(apiContext, schema, data)
}
