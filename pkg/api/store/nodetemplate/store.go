package nodetemplate

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

type Store struct {
	types.Store
	NodePoolLister v3.NodePoolLister
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	pools, err := s.NodePoolLister.List("", labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.Spec.NodeTemplateName == id {
			return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "Template is in use by a node pool.")
		}
	}
	return s.Store.Delete(apiContext, schema, id)
}
