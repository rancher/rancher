package templateversion

import "github.com/rancher/norman/types"

func Wrap(store types.Store) types.Store {
	return &Store{
		Store: store,
	}
}

type Store struct {
	types.Store
}

func (s *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return nil, nil
}
