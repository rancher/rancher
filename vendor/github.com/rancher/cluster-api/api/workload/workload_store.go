package workload

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
)

func ConfigureStore(schemas *types.Schemas) {
	workloadSchema := schemas.Schema(&schema.Version, "workload")

	store := types.Store(&PrefixTypeStore{
		Store: workloadSchema.Store,
	})
	workloadSchema.Store = store

	store = NewAggregateStore(store,
		workloadSchema,
		schemas.Schema(&schema.Version, "deployment"),
		schemas.Schema(&schema.Version, "replicaSet"),
		schemas.Schema(&schema.Version, "replicationController"),
		schemas.Schema(&schema.Version, "daemonSet"),
		schemas.Schema(&schema.Version, "statefulSet"))

	workloadSchema.Store = &workloadStore{
		Store: store,
	}
}

type workloadStore struct {
	types.Store
}

func (w *workloadStore) List(apiContext *types.APIContext, schema *types.Schema, opt types.QueryOptions) ([]map[string]interface{}, error) {
	data, err := w.Store.List(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}

	if opt.Options["hidden"] == "true" {
		return data, err
	}

	var result []map[string]interface{}
	for _, item := range data {
		if item["ownerReferences"] != nil {
			continue
		}
		result = append(result, item)
	}

	return result, nil
}
