package rbac

import v1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"

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

func (l ListPermissionSet) HasAccess(namespace, name string) bool {
	return l[ListPermission{
		Namespace: namespace,
		Name:      name,
	}]
}

type ListPermission struct {
	Namespace string
	Name      string
}

func (l *ListPermissionStore) UserPermissions(subjectName, apiGroup, resource, verb string) ListPermissionSet {
	return getFromIndex(subjectName, apiGroup, resource, verb, l.users)
}

func (l *ListPermissionStore) CheckUserPermission(subjectName, objID, objNamespace, apiGroup, resource, verb string) bool {
	return l.users.validatePermission(subjectName, objID, objNamespace, apiGroup, resource, verb)
}

func (l *ListPermissionStore) CheckUserCanAccess(subjectName, objID, objNamespace, apiGroup, resource, verb string) bool {
	return l.users.userAccessCheck(subjectName, objID, objNamespace, apiGroup, resource, verb)
}

func (l *ListPermissionStore) GroupPermissions(subjectName, apiGroup, resource, verb string) ListPermissionSet {
	return getFromIndex(subjectName, apiGroup, resource, verb, l.groups)
}

func (l *ListPermissionStore) CheckGroupPermission(subjectName, objID, objNamespace, apiGroup, resource, verb string) bool {
	return l.groups.validatePermission(subjectName, objID, objNamespace, apiGroup, resource, verb)
}

func getFromIndex(subjectName, apiGroup, resource, verb string, index *permissionIndex) ListPermissionSet {
	result := ListPermissionSet{}
	for _, value := range index.get(subjectName, apiGroup, resource, verb) {
		result[value] = true
	}
	return result
}
