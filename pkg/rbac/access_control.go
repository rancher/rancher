package rbac

import (
	"context"

	"github.com/rancher/norman/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	v1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac/v1"
)

func NewAccessControl(ctx context.Context, clusterName string, rbacClient v1.Interface) types.AccessControl {
	asl := accesscontrol.NewAccessStore(ctx, true, rbacClient)
	return NewAccessControlWithASL(clusterName, asl)
}

func NewAccessControlWithASL(clusterName string, asl accesscontrol.AccessSetLookup) types.AccessControl {
	return newContextBased(func(ctx *types.APIContext) (types.AccessControl, bool) {
		cache, ok := ctx.Request.Context().Value(contextKey{}).(*accessControlCache)
		if !ok {
			return nil, false
		}

		cache.RLock()
		ac, ok := cache.cache[clusterName]
		if ok {
			if u, ok := ac.(*userCachedAccess); !ok || !u.Expired() {
				cache.RUnlock()
				return ac, true
			}
		}
		cache.RUnlock()

		cache.Lock()
		defer cache.Unlock()
		ac = newUserLookupAccess(ctx, asl)
		cache.cache[clusterName] = ac
		return ac, true
	})
}
