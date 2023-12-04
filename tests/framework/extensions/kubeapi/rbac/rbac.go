package rbac

import (
	"github.com/rancher/rancher/tests/framework/extensions/constants"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName    = "management.cattle.io"
	Version      = "v3"
	localcluster = constants.LocalCluster
)

// RoleGroupVersionResource is the required Group Version Resource for accessing roles in a cluster, using the dynamic client.
var RoleGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: constants.Roles,
}

// ClusterRoleGroupVersionResource is the required Group Version Resource for accessing clusterroles in a cluster, using the dynamic client.
var ClusterRoleGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: constants.ClusterRoles,
}

// RoleBindingGroupVersionResource is the required Group Version Resource for accessing rolebindings in a cluster, using the dynamic client.
var RoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: constants.RoleBindings,
}

// ClusterRoleBindingGroupVersionResource is the required Group Version Resource for accessing clusterrolebindings in a cluster, using the dynamic client.
var ClusterRoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: constants.ClusterRoleBindings,
}

// GlobalRoleGroupVersionResource is the required Group Version Resource for accessing global roles in a rancher server, using the dynamic client.
var GlobalRoleGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: constants.GlobalRoles,
}

// GlobalRoleBindingGroupVersionResource is the required Group Version Resource for accessing clusterrolebindings in a cluster, using the dynamic client.
var GlobalRoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: constants.GlobalRoleBindings,
}

// ClusterRoleTemplateBindingGroupVersionResource is the required Group Version Resource for accessing clusterrolebindings in a cluster, using the dynamic client.
var ClusterRoleTemplateBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: constants.ClusterRoleTemplateBindings,
}

// RoleTemplateGroupVersionResource is the required Group Version Resource for accessing roletemplates in a cluster, using the dynamic client.
var RoleTemplateGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: constants.RoleTemplates,
}

// ProjectRoleTemplateBindingGroupVersionResource is the required Group Version Resource for accessing project role template bindings in a cluster, using the dynamic client.
var ProjectRoleTemplateBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: constants.ProjectRoleTemplateBindings,
}
