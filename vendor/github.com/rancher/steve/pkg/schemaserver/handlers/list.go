package handlers

import (
	"github.com/rancher/steve/pkg/schemaserver/httperror"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

func ByIDHandler(request *types.APIRequest) (types.APIObject, error) {
	if err := request.AccessControl.CanGet(request, request.Schema); err != nil {
		return types.APIObject{}, err
	}

	store := request.Schema.Store
	if store == nil {
		return types.APIObject{}, httperror.NewAPIError(validation.NotFound, "no store found")
	}

	return store.ByID(request, request.Schema, request.Name)
}

func ListHandler(request *types.APIRequest) (types.APIObjectList, error) {
	if request.Name == "" {
		if err := request.AccessControl.CanList(request, request.Schema); err != nil {
			return types.APIObjectList{}, err
		}
	} else {
		if err := request.AccessControl.CanGet(request, request.Schema); err != nil {
			return types.APIObjectList{}, err
		}
	}

	store := request.Schema.Store
	if store == nil {
		return types.APIObjectList{}, httperror.NewAPIError(validation.NotFound, "no store found")
	}

	if request.Link == "" {
		return store.List(request, request.Schema)
	}

	_, err := store.ByID(request, request.Schema, request.Name)
	if err != nil {
		return types.APIObjectList{}, err
	}

	if handler, ok := request.Schema.LinkHandlers[request.Link]; ok {
		handler.ServeHTTP(request.Response, request.Request)
		return types.APIObjectList{}, validation.ErrComplete
	}

	return types.APIObjectList{}, validation.NotFound
}
