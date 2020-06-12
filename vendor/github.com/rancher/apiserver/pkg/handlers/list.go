package handlers

import (
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

func ByIDHandler(request *types.APIRequest) (types.APIObject, error) {
	if err := request.AccessControl.CanGet(request, request.Schema); err != nil {
		return types.APIObject{}, err
	}

	store := request.Schema.Store
	if store == nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, "no store found")
	}

	resp, err := store.ByID(request, request.Schema, request.Name)
	if err != nil {
		return resp, err
	}

	if request.Link != "" {
		if handler, ok := request.Schema.LinkHandlers[request.Link]; ok {
			handler.ServeHTTP(request.Response, request.Request)
			return types.APIObject{}, validation.ErrComplete
		}
	}

	return resp, nil
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
		return types.APIObjectList{}, apierror.NewAPIError(validation.NotFound, "no store found")
	}

	return store.List(request, request.Schema)
}
