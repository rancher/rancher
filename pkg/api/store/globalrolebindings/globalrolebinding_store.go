package grbstore

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

const GrbVersion = "field.cattle.io/grbUpgrade"

func Wrap(store types.Store, grbLister v3.GlobalRoleBindingLister) types.Store {
	return &grbStore{
		Store:     store,
		grbLister: grbLister,
	}
}

type grbStore struct {
	types.Store
	grbLister v3.GlobalRoleBindingLister
}

func (s *grbStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {

	values.PutValue(data, "true", "metadata", "annotations", GrbVersion)

	return s.Store.Create(apiContext, schema, data)
}
