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

// ClusterRoleGroupVersionResource is the required Group Version Resource for accessing clusterroles in a cluster,
// using the dynamic client.
var ClusterRoleGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "clusterroles",
}

// RoleBindingGroupVersionResource is the required Group Version Resource for accessing rolebindings in a cluster,
// using the dynamic client.
var RoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "rolebindings",
}

// ClusterRoleBindingGroupVersionResource is the required Group Version Resource for accessing clusterrolebindings in a cluster,
// using the dynamic client.
var ClusterRoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "clusterrolebindings",
}
