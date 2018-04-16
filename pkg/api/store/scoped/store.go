package scoped

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/client/management/v3"
	mgmtclient "github.com/rancher/types/client/management/v3"
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

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	var project mgmtclient.Project
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &project); err == nil {
		if project.Labels["authz.management.cattle.io/system-project"] == "true" {
			return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "System Project cannot be deleted")
		}
	} else {
		return nil, httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error accessing project [%s]: %v", id, err))
	}
	return s.Store.Delete(apiContext, schema, id)
}
