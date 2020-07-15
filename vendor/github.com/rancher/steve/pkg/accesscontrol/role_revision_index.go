package accesscontrol

import (
	"context"
	"sync"

	rbac "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/kv"
	rbacv1 "k8s.io/api/rbac/v1"
)

type roleRevisionIndex struct {
	roleRevisions sync.Map
}

func newRoleRevision(ctx context.Context, rbac rbac.Interface) *roleRevisionIndex {
	r := &roleRevisionIndex{}
	rbac.Role().OnChange(ctx, "role-revision-indexer", r.onRoleChanged)
	rbac.ClusterRole().OnChange(ctx, "role-revision-indexer", r.onClusterRoleChanged)
	return r
}

func (r *roleRevisionIndex) roleRevision(namespace, name string) string {
	val, _ := r.roleRevisions.Load(roleKey{
		name:      name,
		namespace: namespace,
	})
	revision, _ := val.(string)
	return revision
}

func (r *roleRevisionIndex) onClusterRoleChanged(key string, cr *rbacv1.ClusterRole) (role *rbacv1.ClusterRole, err error) {
	if cr == nil {
		r.roleRevisions.Delete(roleKey{
			name: key,
		})
	} else {
		r.roleRevisions.Store(roleKey{
			name: key,
		}, cr.ResourceVersion)
	}
	return cr, nil
}

func (r *roleRevisionIndex) onRoleChanged(key string, cr *rbacv1.Role) (role *rbacv1.Role, err error) {
	if cr == nil {
		namespace, name := kv.Split(key, "/")
		r.roleRevisions.Delete(roleKey{
			name:      name,
			namespace: namespace,
		})
	} else {
		r.roleRevisions.Store(roleKey{
			name:      cr.Name,
			namespace: cr.Namespace,
		}, cr.ResourceVersion)
	}
	return cr, nil
}
