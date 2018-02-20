package service

import (
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

func New(store types.Store) types.Store {
	return &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			ownerReferences, ok := values.GetSlice(data, "ownerReferences")
			if !ok {
				return data, nil
			}

			for _, ownerReference := range ownerReferences {
				controller, _ := ownerReference["controller"].(bool)
				if controller {
					return nil, nil
				}
			}
			return data, nil
		},
	}
}
