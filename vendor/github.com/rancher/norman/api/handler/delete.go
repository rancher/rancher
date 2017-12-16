package handler

import (
	"net/http"

	"github.com/rancher/norman/types"
)

func DeleteHandler(request *types.APIContext) error {
	store := request.Schema.Store
	if store == nil {
		request.WriteResponse(http.StatusNoContent, nil)
		return nil
	}

	obj, err := store.Delete(request, request.Schema, request.ID)
	if err != nil {
		return err
	}

	if obj == nil {
		request.WriteResponse(http.StatusNoContent, nil)
	} else {
		request.WriteResponse(http.StatusOK, obj)
	}
	return nil
}
