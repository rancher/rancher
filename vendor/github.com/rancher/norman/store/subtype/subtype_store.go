package subtype

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type Store struct {
	types.Store
	subType string
}

func NewSubTypeStore(subType string, store types.Store) *Store {
	return &Store{
		Store:   store,
		subType: subType,
	}
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if data != nil {
		data["kind"] = p.subType
	}
	return p.Store.Create(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if data != nil {
		data["kind"] = convert.Uncapitalize(strings.Replace(p.subType, "namespaced", "", 1))
		data["type"] = data["kind"]
	}
	return p.Store.Update(apiContext, schema, data, id)
}

func (p *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	opt.Conditions = append(opt.Conditions, types.NewConditionFromString("type", types.ModifierEQ, p.subType))
	return p.Store.List(apiContext, schema, opt)
}

func (p *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	opt.Conditions = append(opt.Conditions, types.NewConditionFromString("type", types.ModifierEQ, p.subType))
	return p.Store.Watch(apiContext, schema, opt)
}
