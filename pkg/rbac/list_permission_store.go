package rbac

import "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"

type ListPermissionStore struct {
	users  *permissionIndex
	groups *permissionIndex
}

func NewListPermissionStore(client v1.Interface) *ListPermissionStore {
	users, groups := newIndexes(client)
	return &ListPermissionStore{
		users:  users,
		groups: groups,
	}

}

type ListPermissionSet map[ListPermission]bool

func (l ListPermissionSet) Access(namespace, name string) bool {
	return l[ListPermission{
		Namespace: namespace,
		Name:      name,
	}]
}

type ListPermission struct {
	Namespace string
	Name      string
}

func (l *ListPermissionStore) UserPermissions(name, apiGroup, resource string) ListPermissionSet {
	return getFromIndex(name, apiGroup, resource, l.users)
}

func (l *ListPermissionStore) GroupPermissions(name, apiGroup, resource string) ListPermissionSet {
	return getFromIndex(name, apiGroup, resource, l.groups)
}

func getFromIndex(name, apiGroup, resource string, index *permissionIndex) ListPermissionSet {
	result := ListPermissionSet{}
	for _, value := range index.get(name, apiGroup, resource) {
		result[value] = true
	}
	return result
}
