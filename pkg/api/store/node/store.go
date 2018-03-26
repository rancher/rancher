package node

import (
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/store/workload"
)

type nodeStore struct {
	types.Store
}

func SetupStore(schema *types.Schema) {
	schema.Store = &transform.Store{
		Store: nodeStore{
			Store: schema.Store,
		},
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			workload.SetPublicEnpointsFields(data)
			return data, nil
		},
	}
}

func (n nodeStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	format(data)
	return n.Store.Update(apiContext, schema, data, id)
}

func format(data map[string]interface{}) {
	data["desiredNodeLabels"] = data["labels"]
	data["desiredNodeAnnotations"] = data["annotations"]
}
