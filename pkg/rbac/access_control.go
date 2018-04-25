package rbac

import (
	"net/http"
	"strings"

	"fmt"

	"github.com/rancher/norman/authorization"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/settings"
	publicSchema "github.com/rancher/types/apis/management.cattle.io/v3public/schema"
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

func (a *AccessControl) CanCreate(apiContext *types.APIContext, schema *types.Schema) error {
	fb := func() error {
		return a.AllAccess.CanCreate(apiContext, schema)
	}
	return a.canDo("create", apiContext, nil, schema, true, fb)
}

func (a *AccessControl) CanGet(apiContext *types.APIContext, schema *types.Schema) error {
	fb := func() error {
		return a.AllAccess.CanGet(apiContext, schema)
	}
	return a.canDo("get", apiContext, nil, schema, true, fb)
}

func (a *AccessControl) CanList(apiContext *types.APIContext, schema *types.Schema) error {
	fb := func() error {
		return a.AllAccess.CanList(apiContext, schema)
	}
	return a.canDo("list", apiContext, nil, schema, true, fb)
}

func (a *AccessControl) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	fb := func() error {
		return a.AllAccess.CanUpdate(apiContext, obj, schema)
	}
	return a.canDo("update", apiContext, obj, schema, false, fb)
}

func (a *AccessControl) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	fb := func() error {
		return a.AllAccess.CanDelete(apiContext, obj, schema)
	}
	return a.canDo("delete", apiContext, obj, schema, true, fb)
}

func (a *AccessControl) CanDo(verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	fb := func() error {
		return a.AllAccess.CanDo(verb, apiContext, obj, schema)
	}
	return a.canDo(verb, apiContext, obj, schema, false, fb)
}

func (a *AccessControl) canDo(verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema, checkSetting bool, fallback func() error) error {
	if *apiContext.Version == publicSchema.PublicVersion {
		return nil
	}

	if *apiContext.Version == publicSchema.PublicVersion || schema.Store == nil || len(schema.Store.AuthContext(apiContext)) == 0 || !doDynamicRBAC(checkSetting) {
		return fallback()
	}

	apiGroup := schema.Store.AuthContext(apiContext)["apiGroup"]
	resource := schema.Store.AuthContext(apiContext)["resource"]

	if resource == "" {
		return fallback()
	}

	permset := a.getPermissions(apiContext, apiGroup, resource, verb)

	if a.canAccess(obj, permset) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("can not %v %v ", verb, schema.ID))
}

func doDynamicRBAC(checkSetting bool) bool {
	if checkSetting {
		return settings.DynamicCRUDAuthroization.Get() == "true"
	}
	return true
}

func (a *AccessControl) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}) map[string]interface{} {
	var apiGroup, resource string
	if schema.Store != nil && schema.Store.AuthContext(apiContext) != nil {
		apiGroup = schema.Store.AuthContext(apiContext)["apiGroup"]
		resource = schema.Store.AuthContext(apiContext)["resource"]
	}

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
	namespace, _ := obj["namespaceId"].(string)
	var id string
	if obj != nil {
		id, _ = obj["id"].(string)
	} else {
		id = "*"
	}
	if permset.HasAccess(namespace, "*") || permset.HasAccess("*", "*") {
		return true
	}
	return permset.HasAccess(namespace, strings.TrimPrefix(id, namespace+":"))
}

func (a *AccessControl) FilterList(apiContext *types.APIContext, schema *types.Schema, objs []map[string]interface{}) []map[string]interface{} {
	var apiGroup, resource string
	if schema.Store != nil && schema.Store.AuthContext(apiContext) != nil {
		apiGroup = schema.Store.AuthContext(apiContext)["apiGroup"]
		resource = schema.Store.AuthContext(apiContext)["resource"]
	}

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

//
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

func getUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get("Impersonate-User")
}

func getGroups(apiContext *types.APIContext) []string {
	return apiContext.Request.Header[http.CanonicalHeaderKey("Impersonate-Group")]
}
