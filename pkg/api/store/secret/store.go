package secret

import (
	"strings"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/config"
)

type Store struct {
	types.Store
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	t := convert.ToString(data["kind"])
	t = strings.TrimPrefix(t, "namespaced")
	t = convert.Uncapitalize(t)
	data["kind"] = t
	return s.Store.Create(apiContext, schema, data)
}

func NewNamespacedSecretStore(clientGetter proxy.ClientGetter) *Store {
	return &Store{
		Store: &transform.Store{
			Store: proxy.NewProxyStore(clientGetter,
				config.UserStorageContext,
				[]string{"api"},
				"",
				"v1",
				"Secret",
				"secrets"),
			Transformer: func(apiContext *types.APIContext, data map[string]interface{}) (map[string]interface{}, error) {
				if data == nil {
					return data, nil
				}
				parts := strings.Split(convert.ToString(data["type"]), "/")
				parts[len(parts)-1] = "namespaced" + convert.Capitalize(parts[len(parts)-1])
				data["type"] = strings.Join(parts, "/")
				return data, nil
			},
		},
	}
}
