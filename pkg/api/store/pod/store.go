package pod

import (
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
)

func New(store types.Store) types.Store {
	return &transform.Store{
		Store:       store,
		Transformer: transformer,
	}
}

func transformer(context *types.APIContext, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
	if data == nil {
		return data, nil
	}
	owner := resolveWorkloadID(context, data)
	if owner != "" {
		data["workloadId"] = owner
	}
	return data, nil
}
