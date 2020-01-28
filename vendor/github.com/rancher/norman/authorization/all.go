package authorization

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
)

type AllAccess struct {
}

func (*AllAccess) CanCreate(apiContext *types.APIContext, schema *types.Schema) error {
	if slice.ContainsString(schema.CollectionMethods, http.MethodPost) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not create "+schema.ID)
}

func (*AllAccess) CanGet(apiContext *types.APIContext, schema *types.Schema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodGet) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not get "+schema.ID)
}

func (*AllAccess) CanList(apiContext *types.APIContext, schema *types.Schema) error {
	if slice.ContainsString(schema.CollectionMethods, http.MethodGet) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not list "+schema.ID)
}

func (*AllAccess) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodPut) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not update "+schema.ID)
}

func (*AllAccess) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodDelete) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not delete "+schema.ID)
}

func (*AllAccess) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	if slice.ContainsString(schema.ResourceMethods, verb) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not perform "+verb+" "+schema.ID)
}

func (a *AllAccess) CollectionCanDo(apiGroup, resource, verb string, apiContext *types.APIContext, data []interface{}, schema *types.Schema, fn func(map[string]interface{})) map[string]bool {
	accessMap := make(map[string]bool)
	for _, val := range data {
		obj, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		if err := a.CanDo(apiGroup, resource, verb, apiContext, obj, schema); err != nil {
			continue
		}

		var id, namespace string

		if obj != nil {
			id, _ = obj["id"].(string)
			namespace, _ = obj["namespaceId"].(string)
			if namespace == "" {
				pieces := strings.Split(id, ":")
				if len(pieces) == 2 {
					namespace = pieces[0]
				}
			}
		} else {
			id = "*"
		}
		accessMap[fmt.Sprintf("%s:%s", namespace, id)] = true
	}
	return accessMap
}

func (*AllAccess) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	return obj
}

func (*AllAccess) FilterList(apiContext *types.APIContext, schema *types.Schema, obj []map[string]interface{}, context map[string]string) []map[string]interface{} {
	return obj
}
