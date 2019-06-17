package crd

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
)

type ForgetCRDNotFoundStore struct {
	types.Store
}

func (s *ForgetCRDNotFoundStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	c, err := s.Store.Watch(apiContext, schema, opt)
	apiError, ok := err.(*httperror.APIError)
	if ok && apiError.Code == httperror.NotFound {
		return nil, nil
	}
	return c, err
}
