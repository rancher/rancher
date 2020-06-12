package empty

import (
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

type Store struct {
}

func (e *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return types.APIObject{}, validation.NotFound
}

func (e *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return types.APIObject{}, validation.NotFound
}

func (e *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	return types.APIObjectList{}, validation.NotFound
}

func (e *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	return types.APIObject{}, validation.NotFound
}

func (e *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	return types.APIObject{}, validation.NotFound
}

func (e *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, wr types.WatchRequest) (chan types.APIEvent, error) {
	return nil, nil
}
