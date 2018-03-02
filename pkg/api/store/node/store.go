package node

import "github.com/rancher/norman/types"

type nodeStore struct {
	types.Store
}

func SetupStore(schema *types.Schema) {
	schema.Store = nodeStore{
		Store: schema.Store,
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
