package selector

import (
	"github.com/rancher/apiserver/pkg/types"
	"k8s.io/apimachinery/pkg/labels"
)

type Store struct {
	types.Store
	Selector labels.Selector
}

func (s *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	return s.Store.List(s.addSelector(apiOp), schema)
}

func (s *Store) addSelector(apiOp *types.APIRequest) *types.APIRequest {

	apiOp = apiOp.Clone()
	apiOp.Request = apiOp.Request.Clone(apiOp.Context())
	q := apiOp.Request.URL.Query()
	q.Add("labelSelector", s.Selector.String())
	apiOp.Request.URL.RawQuery = q.Encode()
	return apiOp
}

func (s *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	return s.Store.Watch(s.addSelector(apiOp), schema, w)
}
