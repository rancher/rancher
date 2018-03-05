package namespace

import (
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

func New(store types.Store) types.Store {
	t := &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			anns, _ := data["annotations"].(map[string]interface{})
			if anns["management.cattle.io/system-namespace"] == "true" {
				return nil, nil
			}
			return data, nil
		},
	}

	return &Store{
		Store: t,
	}
}

type Store struct {
	types.Store
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	values.PutValue(data, "{\"conditions\": [{\"type\": \"InitialRolesPopulated\", \"status\": \"False\", \"message\": \"Populating initial roles\"}]}",
		"annotations", "cattle.io/status")
	return p.Store.Create(apiContext, schema, data)
}
