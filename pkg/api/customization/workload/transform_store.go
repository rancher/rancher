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
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			hide := true
			if opt != nil && opt.Options["hidden"] == "true" {
				hide = false
			}
			if hide && data["ownerReferences"] != nil {
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
