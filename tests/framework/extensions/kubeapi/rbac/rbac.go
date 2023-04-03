package rbac

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RoleGroupVersionResource is the required Group Version Resource for accessing roles in a cluster,
// using the dynamic client.
var RoleGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "roles",
}

// RoleBindingGroupVersionResource is the required Group Version Resource for accessing rolebindings in a cluster,
// using the dynamic client.
var RoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "rolebindings",
}
