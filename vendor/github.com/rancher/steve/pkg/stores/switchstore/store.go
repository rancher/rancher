package switchstore

import (
	"github.com/rancher/apiserver/pkg/types"
)

type StorePicker func(apiOp *types.APIRequest, schema *types.APISchema, verb, id string) (types.Store, error)

type Store struct {
	Picker StorePicker
}

func (e *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	s, err := e.Picker(apiOp, schema, "delete", id)
	if err != nil {
		return types.APIObject{}, err
	}
	return s.Delete(apiOp, schema, id)
}

func (e *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	s, err := e.Picker(apiOp, schema, "get", id)
	if err != nil {
		return types.APIObject{}, err
	}
	return s.ByID(apiOp, schema, id)
}

func (e *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	s, err := e.Picker(apiOp, schema, "list", "")
	if err != nil {
		return types.APIObjectList{}, err
	}
	return s.List(apiOp, schema)
}

func (e *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	s, err := e.Picker(apiOp, schema, "create", "")
	if err != nil {
		return types.APIObject{}, err
	}
	return s.Create(apiOp, schema, data)
}

func (e *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	s, err := e.Picker(apiOp, schema, "update", id)
	if err != nil {
		return types.APIObject{}, err
	}
	return s.Update(apiOp, schema, data, id)
}

func (e *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, wr types.WatchRequest) (chan types.APIEvent, error) {
	s, err := e.Picker(apiOp, schema, "watch", "")
	if err != nil {
		return nil, err
	}
	return s.Watch(apiOp, schema, wr)
}
