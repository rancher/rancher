package roletemplate

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	RTVersion    = "cleanup.cattle.io/rtUpgradeCluster"
	OldRTVersion = "field.cattle.io/rtUpgrade"
)

func Wrap(store types.Store, rtLister v3.RoleTemplateLister) types.Store {
	return &rtStore{
		Store:    store,
		rtLister: rtLister,
	}
}

type rtStore struct {
	types.Store

	rtLister v3.RoleTemplateLister
}

func (s *rtStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	roleTemplates, err := s.rtLister.List("", labels.Everything())
	if err != nil {
		return nil, err
	}

	// check if roletemplate is referenced as parent by any other roletemplate
	for _, rt := range roleTemplates {
		for _, parent := range rt.RoleTemplateNames {
			if parent == id {
				return nil, httperror.NewAPIError(httperror.Conflict, fmt.Sprintf("roletemplate [%s] cannot be deleted because roletemplate [%s] inherits from it", id, rt.Name))
			}
		}
	}
	return s.Store.Delete(apiContext, schema, id)
}

func (s *rtStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {

	values.PutValue(data, "true", "metadata", "annotations", RTVersion)

	return s.Store.Create(apiContext, schema, data)
}
