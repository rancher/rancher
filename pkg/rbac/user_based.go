package rbac

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/rancher/norman/authorization"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/auth"
	schema2 "k8s.io/apimachinery/pkg/runtime/schema"
	user2 "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/transport"
)

type accessControlCache struct {
	sync.RWMutex
	cache map[string]types.AccessControl
}

func NewAccessControlHandler() auth.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			val := &accessControlCache{
				cache: map[string]types.AccessControl{},
			}
			ctx := context.WithValue(req.Context(), contextKey{}, val)
			req = req.WithContext(ctx)
			next.ServeHTTP(rw, req)
		})
	}
}

func newUserLookupAccess(ctx *types.APIContext, accessStore accesscontrol.AccessSetLookup) types.AccessControl {
	userName := ctx.Request.Header.Get(transport.ImpersonateUserHeader)
	groups := ctx.Request.Header[transport.ImpersonateGroupHeader]
	user := &user2.DefaultInfo{
		Name:   userName,
		Groups: groups,
	}
	accessSet := accessStore.AccessFor(user)
	return &userCachedAccess{
		access: accessSet,
	}
}

type userCachedAccess struct {
	authorization.AllAccess
	expired bool
	access  *accesscontrol.AccessSet
}

func (a *userCachedAccess) Expire(apiContext *types.APIContext, schema *types.Schema) {
	a.expired = true
}

func (a *userCachedAccess) Expired() bool {
	return a.expired
}

func (a *userCachedAccess) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	name, ns := getNameAndNS(obj)
	if !a.access.Grants(verb, schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	}, ns, name) {
		return httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("can not %v %v ", verb, schema.ID))
	}
	return nil
}

func (a *userCachedAccess) FilterList(apiContext *types.APIContext, schema *types.Schema, objs []map[string]interface{}, context map[string]string) []map[string]interface{} {
	apiGroup := context["apiGroup"]
	resource := context["resource"]

	if resource == "" {
		return objs
	}

	if len(objs) == 0 {
		return objs
	}

	gr := schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	}

	result := make([]map[string]interface{}, 0, len(objs))
	for _, obj := range objs {
		name, ns := getNameAndNS(obj)
		if a.access.Grants("list", gr, ns, name) || a.access.Grants("get", gr, ns, name) {
			result = append(result, obj)
		}
	}
	return result
}

func (a *userCachedAccess) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	apiGroup := context["apiGroup"]
	resource := context["resource"]

	if resource == "" {
		return obj
	}

	name, ns := getNameAndNS(obj)
	if a.access.Grants("list", schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	}, ns, name) {
		return obj
	}

	if a.access.Grants("get", schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	}, ns, name) {
		return obj
	}

	return nil
}

func getNameAndNS(obj map[string]interface{}) (string, string) {
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
	}

	id = strings.TrimPrefix(id, namespace+":")
	return id, namespace
}
