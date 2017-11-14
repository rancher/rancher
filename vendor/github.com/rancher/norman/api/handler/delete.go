package handler

import (
	"net/http"

	"github.com/rancher/norman/types"
)

func DeleteHandler(request *types.APIContext) error {
	store := request.Schema.Store
	if store != nil {
		err := store.Delete(request, request.Schema, request.ID)
		if err != nil {
			return err
		}
	}

	request.WriteResponse(http.StatusNoContent, nil)
	return nil
}
