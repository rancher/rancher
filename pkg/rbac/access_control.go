package rbac

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/authorization"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
)

type AccessControl struct {
	authorization.AllAccess
	permissionStore *ListPermissionStore
}

func NewAccessControl(rbacClient v1.Interface) *AccessControl {
	permissionStore := NewListPermissionStore(rbacClient)
	return &AccessControl{
		permissionStore: permissionStore,
	}
}

func (a *AccessControl) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	var id string
	var namespace string

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

	if a.validatePermission(apiContext, id, namespace, apiGroup, resource, verb) {
		return nil
	}

	return httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("can not %v %v ", verb, schema.ID))
}

func (a *AccessControl) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	apiGroup := context["apiGroup"]
	resource := context["resource"]

	if resource == "" {
		return obj
	}

	permset := a.getPermissions(apiContext, apiGroup, resource, "list")

	if a.canAccess(obj, permset) {
		return obj
	}
	return nil
}

func (a *AccessControl) canAccess(obj map[string]interface{}, permset ListPermissionSet) bool {
	var id string
	var namespace string

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

	if permset.HasAccess(namespace, "*") || permset.HasAccess("*", "*") {
		return true
	}
	return permset.HasAccess(namespace, strings.TrimPrefix(id, namespace+":"))
}

func (a *AccessControl) FilterList(apiContext *types.APIContext, schema *types.Schema, objs []map[string]interface{}, context map[string]string) []map[string]interface{} {
	apiGroup := context["apiGroup"]
	resource := context["resource"]

	if resource == "" {
		return objs
	}

	permset := a.getPermissions(apiContext, apiGroup, resource, "list")

	result := make([]map[string]interface{}, 0, len(objs))

	all := permset.HasAccess("*", "*")

	for _, obj := range objs {
		if all {
			result = append(result, obj)
		} else if a.canAccess(obj, permset) {
			result = append(result, obj)
		}
	}

	return result
}

func (a *AccessControl) getPermissions(context *types.APIContext, apiGroup, resource, verb string) ListPermissionSet {
	permset := a.permissionStore.UserPermissions(getUser(context), apiGroup, resource, verb)
	if permset == nil {
		permset = ListPermissionSet{}
	}
	for _, group := range getGroups(context) {
		for k, v := range a.permissionStore.GroupPermissions(group, apiGroup, resource, verb) {
			permset[k] = v
		}
	}

	return permset
}

func (a *AccessControl) validatePermission(context *types.APIContext, id, namespace, apiGroup, resource, verb string) bool {
	if a.permissionStore.CheckUserPermission(getUser(context), id, namespace, apiGroup, resource, verb) {
		return true
	}

	for _, group := range getGroups(context) {
		if a.permissionStore.CheckGroupPermission(group, id, namespace, apiGroup, resource, verb) {
			return true
		}
	}
	return false
}

func getUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get("Impersonate-User")
}

func getGroups(apiContext *types.APIContext) []string {
	return apiContext.Request.Header[http.CanonicalHeaderKey("Impersonate-Group")]
}
