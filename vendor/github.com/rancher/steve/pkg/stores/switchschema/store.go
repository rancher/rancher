package switchschema

import (
	"github.com/rancher/apiserver/pkg/types"
)

type Store struct {
	Schema *types.APISchema
}

func (e *Store) Delete(apiOp *types.APIRequest, oldSchema *types.APISchema, id string) (types.APIObject, error) {
	obj, err := e.Schema.Store.Delete(apiOp, e.Schema, id)
	obj.Type = oldSchema.ID
	return obj, err
}

func (e *Store) ByID(apiOp *types.APIRequest, oldSchema *types.APISchema, id string) (types.APIObject, error) {
	obj, err := e.Schema.Store.ByID(apiOp, e.Schema, id)
	obj.Type = oldSchema.ID
	return obj, err
}

func (e *Store) List(apiOp *types.APIRequest, oldSchema *types.APISchema) (types.APIObjectList, error) {
	obj, err := e.Schema.Store.List(apiOp, e.Schema)
	for i := range obj.Objects {
		obj.Objects[i].Type = oldSchema.ID
	}
	return obj, err
}

func (e *Store) Create(apiOp *types.APIRequest, oldSchema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	obj, err := e.Schema.Store.Create(apiOp, e.Schema, data)
	obj.Type = oldSchema.ID
	return obj, err
}

func (e *Store) Update(apiOp *types.APIRequest, oldSchema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	obj, err := e.Schema.Store.Update(apiOp, e.Schema, data, id)
	obj.Type = oldSchema.ID
	return obj, err
}

func (e *Store) Watch(apiOp *types.APIRequest, oldSchema *types.APISchema, wr types.WatchRequest) (chan types.APIEvent, error) {
	c, err := e.Schema.Store.Watch(apiOp, e.Schema, wr)
	if err != nil || c == nil {
		return c, err
	}

	result := make(chan types.APIEvent)
	go func() {
		defer close(result)
		for obj := range c {
			if obj.Object.Type == e.Schema.ID {
				obj.Object.Type = oldSchema.ID
			}
			result <- obj
		}
	}()

	return result, nil
}
