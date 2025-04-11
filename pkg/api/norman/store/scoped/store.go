package scoped

import (
	"strings"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

type Store struct {
	types.Store
	key          string
	projectCache v3.ProjectLister
}

func NewScopedStore(key string, store types.Store, pLister v3.ProjectLister) *Store {
	return &Store{
		Store: &transform.Store{
			Store: store,
			Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
				if data == nil {
					return data, nil
				}
				v := convert.ToString(data[key])
				if !strings.HasSuffix(v, ":"+convert.ToString(data[client.ProjectFieldNamespaceId])) && v != strings.Replace(convert.ToString(data[client.ProjectFieldNamespaceId]), "-", ":", 1) {
					data[key] = data[client.ProjectFieldNamespaceId]
				}

				data[client.ProjectFieldNamespaceId] = nil
				return data, nil
			},
		},
		key:          key,
		projectCache: pLister,
	}
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if data == nil {
		return s.Store.Create(apiContext, schema, data)
	}

	clusterName, projectName, isProject := strings.Cut(convert.ToString(data[s.key]), ":")
	if isProject {
		p, err := s.projectCache.Get(clusterName, projectName)
		if err != nil {
			return nil, err
		}
		data[client.ProjectFieldNamespaceId] = p.GetProjectBackingNamespace()
	} else {
		data[client.ProjectFieldNamespaceId] = data[s.key]
	}

	return s.Store.Create(apiContext, schema, data)
}
