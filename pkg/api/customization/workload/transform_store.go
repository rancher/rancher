package workload

import (
	"strings"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
)

func New(store types.Store) types.Store {
	return &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}) (map[string]interface{}, error) {
			if data["ownerReferences"] != nil {
				return nil, nil
			}
			typeName := definition.GetType(data)
			id, _ := data["id"].(string)
			if typeName != "" && id != "" {
				data["id"] = strings.ToLower(typeName) + ":" + id
			}
			return data, nil
		},
	}
}
