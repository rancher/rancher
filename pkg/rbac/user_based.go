package rbac

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/authorization"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/auth"
	schema2 "k8s.io/apimachinery/pkg/runtime/schema"
	user2 "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/transport"
)

func NewAccessControlHandler(as accesscontrol.AccessSetLookup) auth.Middleware {
	return func(rw http.ResponseWriter, req *http.Request, next http.Handler) {
		ac := newUserLookupAccess(as)
		ctx := context.WithValue(req.Context(), contextKey{}, ac)
		req = req.WithContext(ctx)
		next.ServeHTTP(rw, req)
	}
}

func newUserLookupAccess(accessStore accesscontrol.AccessSetLookup) types.AccessControl {
	return &lazyContext{
		factory: func(ctx *types.APIContext) types.AccessControl {
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
		},
	}
}

type userCachedAccess struct {
	authorization.AllAccess
	access *accesscontrol.AccessSet
}

func (a *userCachedAccess) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	set := a.access.AccessListFor(verb, schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	})
	name, ns := getNameAndNS(obj)
	if !set.Grants(ns, name) {
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

	listSet := a.access.AccessListFor("list", schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	})
	getSet := a.access.AccessListFor("get", schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	})

	result := make([]map[string]interface{}, 0, len(objs))
	for _, obj := range objs {
		name, ns := getNameAndNS(obj)
		if listSet.Grants(ns, name) || getSet.Grants(ns, name) {
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

	set := a.access.AccessListFor("list", schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	})

	name, ns := getNameAndNS(obj)
	if set.Grants(ns, name) {
		return obj
	}

	set = a.access.AccessListFor("get", schema2.GroupResource{
		Group:    apiGroup,
		Resource: resource,
	})
	if set.Grants(ns, name) {
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
