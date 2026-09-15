package projects

import (
	"fmt"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schema/converter"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// steveStore serves projects from steve's own store.
type steveStore struct {
	schemaFactory schema.Factory
}

func newSteveStore(schemaFactory schema.Factory) *steveStore {
	return &steveStore{schemaFactory: schemaFactory}
}

// schemaFor returns steve's schema for the type s describes.
func (u *steveStore) schemaFor(apiOp *types.APIRequest, s *types.APISchema) (*types.APISchema, error) {
	user, ok := request.UserFrom(apiOp.Context())
	if !ok {
		return nil, fmt.Errorf("could not find user in request")
	}

	schemas, err := u.schemaFactory.Schemas(user)
	if err != nil {
		return nil, err
	}

	id := converter.GVKToSchemaID(attributes.GVK(s))
	apiSchema := schemas.LookupSchema(id)
	if apiSchema == nil || apiSchema.Store == nil {
		return nil, fmt.Errorf("no store for %s", id)
	}

	return apiSchema, nil
}

func (u *steveStore) ByID(apiOp *types.APIRequest, s *types.APISchema, id string) (types.APIObject, error) {
	apiSchema, err := u.schemaFor(apiOp, s)
	if err != nil {
		return types.APIObject{}, err
	}
	return apiSchema.Store.ByID(apiOp, apiSchema, id)
}

func (u *steveStore) List(apiOp *types.APIRequest, s *types.APISchema) (types.APIObjectList, error) {
	apiSchema, err := u.schemaFor(apiOp, s)
	if err != nil {
		return types.APIObjectList{}, err
	}
	return apiSchema.Store.List(apiOp, apiSchema)
}

func (u *steveStore) Create(apiOp *types.APIRequest, s *types.APISchema, data types.APIObject) (types.APIObject, error) {
	apiSchema, err := u.schemaFor(apiOp, s)
	if err != nil {
		return types.APIObject{}, err
	}
	return apiSchema.Store.Create(apiOp, apiSchema, data)
}

func (u *steveStore) Update(apiOp *types.APIRequest, s *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	apiSchema, err := u.schemaFor(apiOp, s)
	if err != nil {
		return types.APIObject{}, err
	}
	return apiSchema.Store.Update(apiOp, apiSchema, data, id)
}

func (u *steveStore) Delete(apiOp *types.APIRequest, s *types.APISchema, id string) (types.APIObject, error) {
	apiSchema, err := u.schemaFor(apiOp, s)
	if err != nil {
		return types.APIObject{}, err
	}
	return apiSchema.Store.Delete(apiOp, apiSchema, id)
}

func (u *steveStore) Watch(apiOp *types.APIRequest, s *types.APISchema, wr types.WatchRequest) (chan types.APIEvent, error) {
	apiSchema, err := u.schemaFor(apiOp, s)
	if err != nil {
		return nil, err
	}
	return apiSchema.Store.Watch(apiOp, apiSchema, wr)
}
