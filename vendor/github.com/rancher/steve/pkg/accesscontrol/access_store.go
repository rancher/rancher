package accesscontrol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/kv"
	k8srbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/apiserver/pkg/authentication/user"
)

type AccessSetLookup interface {
	AccessFor(user user.Info) *AccessSet
}

type AccessStore struct {
	users         *policyRuleIndex
	groups        *policyRuleIndex
	roleRevisions sync.Map
	cache         *cache.LRUExpireCache
}

type roleKey struct {
	namespace string
	name      string
}

func NewAccessStore(ctx context.Context, cacheResults bool, rbac v1.Interface) *AccessStore {
	as := &AccessStore{
		users:  newPolicyRuleIndex(true, rbac),
		groups: newPolicyRuleIndex(false, rbac),
	}
	rbac.Role().OnChange(ctx, "role-revision-indexer", as.onRoleChanged)
	rbac.ClusterRole().OnChange(ctx, "role-revision-indexer", as.onClusterRoleChanged)
	if cacheResults {
		as.cache = cache.NewLRUExpireCache(1000)
	}
	return as
}

func (l *AccessStore) onClusterRoleChanged(key string, cr *k8srbac.ClusterRole) (role *k8srbac.ClusterRole, err error) {
	if cr == nil {
		l.roleRevisions.Delete(roleKey{
			name: key,
		})
	} else {
		l.roleRevisions.Store(roleKey{
			name: key,
		}, cr.ResourceVersion)
	}
	return cr, nil
}

func (l *AccessStore) onRoleChanged(key string, cr *k8srbac.Role) (role *k8srbac.Role, err error) {
	if cr == nil {
		namespace, name := kv.Split(key, "/")
		l.roleRevisions.Delete(roleKey{
			name:      name,
			namespace: namespace,
		})
	} else {
		l.roleRevisions.Store(roleKey{
			name:      cr.Name,
			namespace: cr.Namespace,
		}, cr.ResourceVersion)
	}
	return cr, nil
}

func (l *AccessStore) AccessFor(user user.Info) *AccessSet {
	var cacheKey string
	if l.cache != nil {
		cacheKey = l.CacheKey(user)
		val, ok := l.cache.Get(cacheKey)
		if ok {
			as, _ := val.(*AccessSet)
			return as
		}
	}

	result := l.users.get(user.GetName())
	for _, group := range user.GetGroups() {
		result.Merge(l.groups.get(group))
	}

	if l.cache != nil {
		result.ID = cacheKey
		l.cache.Add(cacheKey, result, 24*time.Hour)
	}

	return result
}

func (l *AccessStore) CacheKey(user user.Info) string {
	roles := map[roleKey]struct{}{}
	l.users.addRolesToMap(roles, user.GetName())
	for _, group := range user.GetGroups() {
		l.groups.addRolesToMap(roles, group)
	}

	revs := make([]string, 0, len(roles))
	for roleKey := range roles {
		val, _ := l.roleRevisions.Load(roleKey)
		rev, _ := val.(string)
		revs = append(revs, roleKey.namespace+"/"+roleKey.name+":"+rev)
	}

	sort.Strings(revs)
	d := sha256.New()
	for _, rev := range revs {
		d.Write([]byte(rev))
	}
	return hex.EncodeToString(d.Sum(nil))
}
