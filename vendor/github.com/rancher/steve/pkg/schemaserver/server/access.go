package server

import (
	"net/http"

	"github.com/rancher/steve/pkg/schemaserver/httperror"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/rancher/wrangler/pkg/slice"
)

type SchemaBasedAccess struct {
}

func (*SchemaBasedAccess) CanCreate(apiOp *types.APIRequest, schema *types.APISchema) error {
	if slice.ContainsString(schema.CollectionMethods, http.MethodPost) {
		return nil
	}
	return httperror.NewAPIError(validation.PermissionDenied, "can not create "+schema.ID)
}

func (*SchemaBasedAccess) CanGet(apiOp *types.APIRequest, schema *types.APISchema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodGet) {
		return nil
	}
	return httperror.NewAPIError(validation.PermissionDenied, "can not get "+schema.ID)
}

func (*SchemaBasedAccess) CanList(apiOp *types.APIRequest, schema *types.APISchema) error {
	if slice.ContainsString(schema.CollectionMethods, http.MethodGet) {
		return nil
	}
	return httperror.NewAPIError(validation.PermissionDenied, "can not list "+schema.ID)
}

func (*SchemaBasedAccess) CanUpdate(apiOp *types.APIRequest, obj types.APIObject, schema *types.APISchema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodPut) {
		return nil
	}
	return httperror.NewAPIError(validation.PermissionDenied, "can not update "+schema.ID)
}

func (*SchemaBasedAccess) CanDelete(apiOp *types.APIRequest, obj types.APIObject, schema *types.APISchema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodDelete) {
		return nil
	}
	return httperror.NewAPIError(validation.PermissionDenied, "can not delete "+schema.ID)
}

func (a *SchemaBasedAccess) CanWatch(apiOp *types.APIRequest, schema *types.APISchema) error {
	return a.CanList(apiOp, schema)
}

func (*SchemaBasedAccess) CanAction(apiOp *types.APIRequest, schema *types.APISchema, name string) error {
	if _, ok := schema.ActionHandlers[name]; ok {
		return httperror.NewAPIError(validation.PermissionDenied, "no such action "+name)
	}
	return nil
}
