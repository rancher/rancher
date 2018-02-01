package authorization

import (
	"net/http"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
)

type AllAccess struct {
}

func (*AllAccess) CanCreate(apiContext *types.APIContext, schema *types.Schema) bool {
	return slice.ContainsString(schema.CollectionMethods, http.MethodPost)
}

func (*AllAccess) CanGet(apiContext *types.APIContext, schema *types.Schema) bool {
	return slice.ContainsString(schema.ResourceMethods, http.MethodGet)
}

func (*AllAccess) CanList(apiContext *types.APIContext, schema *types.Schema) bool {
	return slice.ContainsString(schema.CollectionMethods, http.MethodGet)
}

func (*AllAccess) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) bool {
	return slice.ContainsString(schema.ResourceMethods, http.MethodPut)
}

func (*AllAccess) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) bool {
	return slice.ContainsString(schema.ResourceMethods, http.MethodDelete)
}

func (*AllAccess) Filter(apiContext *types.APIContext, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	return obj
}

func (*AllAccess) FilterList(apiContext *types.APIContext, obj []map[string]interface{}, context map[string]string) []map[string]interface{} {
	return obj
}
