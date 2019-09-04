package roletemplate

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

const RTVersion = "field.cattle.io/rtUpgrade"

func Wrap(store types.Store, rtLister v3.RoleTemplateLister) types.Store {
	return &rtStore{
		Store: store,
	}
}

type rtStore struct {
	types.Store
}

func (s *rtStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {

	values.PutValue(data, "true", "metadata", "annotations", RTVersion)

	return s.Store.Create(apiContext, schema, data)
}
