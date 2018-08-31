package namespace

import (
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

func New(store types.Store) types.Store {
	t := &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
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
	if _, ok := data["resourceQuota"]; ok {
		values.PutValue(data, "{\"conditions\": [{\"type\": \"InitialRolesPopulated\", \"status\": \"Unknown\", \"message\": \"Populating initial roles\"},{\"type\": \"ResourceQuotaValidated\", \"status\": \"Unknown\", \"message\": \"Validating resource quota\"}]}",
			"annotations", "cattle.io/status")
	} else {
		values.PutValue(data, "{\"conditions\": [{\"type\": \"InitialRolesPopulated\", \"status\": \"Unknown\", \"message\": \"Populating initial roles\"}]}",
			"annotations", "cattle.io/status")
	}

	if err := p.validateResourceQuota(apiContext, schema, data, ""); err != nil {
		return nil, err
	}

	return p.Store.Create(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := p.validateResourceQuota(apiContext, schema, data, id); err != nil {
		return nil, err
	}

	return p.Store.Update(apiContext, schema, data, id)
}

func (p *Store) validateResourceQuota(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) error {
	return nil
}
