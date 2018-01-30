package secret

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type store struct {
	types.Store
	subType string
}

func NewSecretSubtypeStore(subType string, s types.Store) *store {
	return &store{
		Store:   s,
		subType: subType,
	}
}

func (p *store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if data != nil {
		data["kind"] = p.subType
	}
	return p.Store.Create(apiContext, schema, data)
}

func (p *store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if data != nil {
		data["kind"] = convert.Uncapitalize(strings.Replace(p.subType, "namespaced", "", 1))
		data["type"] = data["kind"]
	}
	return p.Store.Update(apiContext, schema, data, id)
}

func (p *store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	opt.Conditions = append(opt.Conditions, types.NewConditionFromString("type", types.ModifierEQ, p.subType))
	return p.Store.List(apiContext, schema, opt)
}

func (p *store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	opt.Conditions = append(opt.Conditions, types.NewConditionFromString("type", types.ModifierEQ, p.subType))
	return p.Store.Watch(apiContext, schema, opt)
}
