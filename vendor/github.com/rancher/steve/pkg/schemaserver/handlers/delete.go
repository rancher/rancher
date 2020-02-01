package handlers

import (
	"github.com/rancher/steve/pkg/schemaserver/httperror"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

func DeleteHandler(request *types.APIRequest) (types.APIObject, error) {
	if err := request.AccessControl.CanDelete(request, types.APIObject{}, request.Schema); err != nil {
		return types.APIObject{}, err
	}

	store := request.Schema.Store
	if store == nil {
		return types.APIObject{}, httperror.NewAPIError(validation.NotFound, "no store found")
	}

	return store.Delete(request, request.Schema, request.Name)
}
