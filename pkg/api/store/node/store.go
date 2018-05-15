package node

import (
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
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

func (n nodeStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	setName := false
	for _, cond := range opt.Conditions {
		if cond.Field == "name" && cond.ToCondition().Modifier == types.ModifierEQ {
			setName = true
			break
		}
	}

	datas, err := n.Store.List(apiContext, schema, opt)
	if err != nil || !setName {
		return datas, err
	}

	for _, data := range datas {
		if !convert.IsEmpty(data["name"]) {
			continue
		}

		if !convert.IsEmpty(data["nodeName"]) {
			data["name"] = data["nodeName"]
			continue
		}

		if !convert.IsEmpty(data["requestedHostname"]) {
			data["name"] = data["requestedHostname"]
			continue
		}
	}

	return datas, err
}

func (n nodeStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	format(data)
	return n.Store.Update(apiContext, schema, data, id)
}

func format(data map[string]interface{}) {
	data["desiredNodeLabels"] = data["labels"]
	data["desiredNodeAnnotations"] = data["annotations"]
}
