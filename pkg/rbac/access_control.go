package rbac

import (
	"net/http"
	"strings"

	"github.com/rancher/norman/authorization"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
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

func (a *AccessControl) Filter(apiContext *types.APIContext, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	apiGroup := context["apiGroup"]
	resource := context["resource"]

	if resource == "" {
		return obj
	}

	permset := a.getPermissions(apiContext, apiGroup, resource)

	if a.canAccess(obj, permset) {
		return obj
	}
	return nil
}

func (a *AccessControl) canAccess(obj map[string]interface{}, permset ListPermissionSet) bool {
	namespace, _ := obj["namespaceId"].(string)
	id, _ := obj["id"].(string)
	if permset.Access(namespace, "*") || permset.Access("*", "*") {
		return true
	}
	return permset.Access(namespace, strings.TrimPrefix(id, namespace+":"))
}

func (a *AccessControl) FilterList(apiContext *types.APIContext, objs []map[string]interface{}, context map[string]string) []map[string]interface{} {
	apiGroup := context["apiGroup"]
	resource := context["resource"]

	if resource == "" {
		return objs
	}

	permset := a.getPermissions(apiContext, apiGroup, resource)

	result := make([]map[string]interface{}, 0, len(objs))

	all := permset.Access("*", "*")

	for _, obj := range objs {
		if all {
			result = append(result, obj)
		} else if a.canAccess(obj, permset) {
			result = append(result, obj)
		}
	}

	return result
}

func (a *AccessControl) getPermissions(context *types.APIContext, apiGroup, resource string) ListPermissionSet {
	permset := a.permissionStore.UserPermissions(getUser(context), apiGroup, resource)
	if permset == nil {
		permset = ListPermissionSet{}
	}
	for _, group := range getGroups(context) {
		for k, v := range a.permissionStore.GroupPermissions(group, apiGroup, resource) {
			permset[k] = v
		}
	}

	return permset
}

func getUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get("Impersonate-User")
}

func getGroups(apiContext *types.APIContext) []string {
	return apiContext.Request.Header[http.CanonicalHeaderKey("Impersonate-Group")]
}
