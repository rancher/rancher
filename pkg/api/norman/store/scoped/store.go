package scoped

import (
	"strings"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Store struct {
	types.Store
	key           string
	projectClient v3.ProjectInterface
}

func NewScopedStore(key string, store types.Store, pClient v3.ProjectInterface) *Store {
	return &Store{
		Store: &transform.Store{
			Store: store,
			Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
				if data == nil {
					return data, nil
				}

				v := convert.ToString(data[key])
				namespaceId := convert.ToString(data[client.ProjectFieldNamespaceId])
				if !strings.HasSuffix(v, ":"+namespaceId) && strings.Replace(v, ":", "-", 1) != namespaceId {
					data[key] = data[client.ProjectFieldNamespaceId]
				}

				data[client.ProjectFieldNamespaceId] = nil
				return data, nil
			},
		},
		key:           key,
		projectClient: pClient,
	}
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if data == nil {
		return s.Store.Create(apiContext, schema, data)
	}

	clusterName, projectName, isProject := strings.Cut(convert.ToString(data[s.key]), ":")
	if isProject {
		p, err := s.projectClient.GetNamespaced(clusterName, projectName, v1.GetOptions{})
		if err != nil {
			return nil, err
		}

		projectNamespace := projectName
		if p.Status.BackingNamespace != "" {
			projectNamespace = p.Status.BackingNamespace
		}
		data[client.ProjectFieldNamespaceId] = projectNamespace
	} else {
		data[client.ProjectFieldNamespaceId] = clusterName
	}

	return s.Store.Create(apiContext, schema, data)
}
