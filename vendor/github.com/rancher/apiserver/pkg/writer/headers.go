package writer

import (
	"github.com/rancher/apiserver/pkg/types"
)

func AddCommonResponseHeader(apiOp *types.APIRequest) error {
	addExpires(apiOp)
	return addSchemasHeader(apiOp)
}

func addSchemasHeader(apiOp *types.APIRequest) error {
	schema := apiOp.Schemas.Schemas["schema"]
	if schema == nil {
		return nil
	}

	apiOp.Response.Header().Set("X-Api-Schemas", apiOp.URLBuilder.Collection(schema))
	return nil
}

func addExpires(apiOp *types.APIRequest) {
	apiOp.Response.Header().Set("Expires", "Wed 24 Feb 1982 18:42:00 GMT")
}
