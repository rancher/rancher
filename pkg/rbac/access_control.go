package rbac

import (
	"context"
	"net/http"
	"sync"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/steve/pkg/accesscontrol"
	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/rbac/v1"
)

func NewAccessControl(ctx context.Context, clusterName string, rbacClient v1.Interface) types.AccessControl {
	asl := accesscontrol.NewAccessStore(ctx, features.Steve.Enabled(), rbacClient)
	return NewAccessControlWithASL(clusterName, rbacClient, asl)
}

func isSubscribe(ctx *types.APIContext) bool {
	return ctx.Request.Method == http.MethodGet && ctx.Type == "subscribe"
}

func NewAccessControlWithASL(clusterName string, rbacClient v1.Interface, asl accesscontrol.AccessSetLookup) types.AccessControl {
	var cacheLock sync.RWMutex
	return newContextBased(func(ctx *types.APIContext) (types.AccessControl, bool) {
		cache, ok := ctx.Request.Context().Value(contextKey{}).(map[string]types.AccessControl)
		if !ok {
			return nil, false
		}

		if !isSubscribe(ctx) {
			cacheLock.RLock()
			ac, ok := cache[clusterName]
			if ok {
				if u, ok := ac.(*userCachedAccess); !ok || !u.Expired() {
					cacheLock.RUnlock()
					return ac, true
				}
			}
			cacheLock.RUnlock()
		}

		cacheLock.Lock()
		defer cacheLock.Unlock()
		ac := newUserLookupAccess(ctx, asl)
		cache[clusterName] = ac
		return ac, true
	})
}
