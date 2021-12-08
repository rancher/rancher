package nocondition

import (
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

//NewWrapper returns a wrapper store which sets your object to transitioning state if your object has no conditions.
func NewWrapper(state, message string) func(types.Store) types.Store {
	return func(store types.Store) types.Store {
		return &transform.Store{
			Store: store,
			Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
				conditions, ok := values.GetSlice(data, "conditions")
				if !ok || len(conditions) == 0 {
					values.PutValue(data, state, "state")
					values.PutValue(data, "yes", "transitioning")
					values.PutValue(data, message, "transitioningMessage")
				}
				return data, nil
			},
		}
	}
}
