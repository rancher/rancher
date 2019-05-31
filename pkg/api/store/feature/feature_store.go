package feature

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
)

type Store struct {
	types.Store
}

func New(store types.Store) types.Store {
	return &Store{
		store,
	}
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "cannot delete features")
}
