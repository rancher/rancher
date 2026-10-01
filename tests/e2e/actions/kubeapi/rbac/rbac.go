package rbac

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName    = "management.cattle.io"
	Version      = "v3"
	LocalCluster = "local"
)

// RoleGroupVersionResource is the required Group Version Resource for accessing roles in a cluster, using the dynamic client.
var RoleGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "roles",
}

// ClusterRoleGroupVersionResource is the required Group Version Resource for accessing clusterroles in a cluster, using the dynamic client.
var ClusterRoleGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "clusterroles",
}

// RoleBindingGroupVersionResource is the required Group Version Resource for accessing rolebindings in a cluster, using the dynamic client.
var RoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "rolebindings",
}

// ClusterRoleBindingGroupVersionResource is the required Group Version Resource for accessing clusterrolebindings in a cluster, using the dynamic client.
var ClusterRoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "clusterrolebindings",
}

// GlobalRoleGroupVersionResource is the required Group Version Resource for accessing global roles in a rancher server, using the dynamic client.
var GlobalRoleGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: "globalroles",
}

// GlobalRoleBindingGroupVersionResource is the required Group Version Resource for accessing clusterrolebindings in a cluster, using the dynamic client.
var GlobalRoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: "globalrolebindings",
}

// ClusterRoleTemplateBindingGroupVersionResource is the required Group Version Resource for accessing clusterrolebindings in a cluster, using the dynamic client.
var ClusterRoleTemplateBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: "clusterroletemplatebindings",
}

// RoleTemplateGroupVersionResource is the required Group Version Resource for accessing roletemplates in a cluster, using the dynamic client.
var RoleTemplateGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: "roletemplates",
}

// ProjectRoleTemplateBindingGroupVersionResource is the required Group Version Resource for accessing projectroletemplatebindings in a cluster, using the dynamic client.
var ProjectRoleTemplateBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: "projectroletemplatebindings",
}
