package rbac

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GetClusterRoleByName is a helper function that uses the dynamic client to create a get a specific clusterrole for a specific cluster.
func GetClusterRoleByName(client *rancher.Client, clusterID, clusterRoleName string) (*rbacv1.ClusterRole, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}
	clusterRoleResource := dynamicClient.Resource(ClusterRoleGroupVersionResource)

	unstructuredResp, err := clusterRoleResource.Get(context.TODO(), clusterRoleName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newRole := &rbacv1.ClusterRole{}
	err = scheme.Scheme.Convert(unstructuredResp, newRole, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newRole, nil
}

// GetRoleByName is a helper function that uses the dynamic client to create a get a specific role for a specific cluster.
func GetRoleByName(client *rancher.Client, clusterID, namespace, roleName string) (*rbacv1.Role, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}
	roleResource := dynamicClient.Resource(RoleGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := roleResource.Get(context.TODO(), roleName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newRole := &rbacv1.Role{}
	err = scheme.Scheme.Convert(unstructuredResp, newRole, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newRole, nil
}

// GetRoleBindingByName is a helper function that uses the dynamic client to create a get a specific rolebinding for a specific cluster.
func GetRoleBindingByName(client *rancher.Client, clusterID, namespace, roleBindingName string) (*rbacv1.RoleBinding, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}
	roleBindingResource := dynamicClient.Resource(RoleBindingGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := roleBindingResource.Get(context.TODO(), roleBindingName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newRoleBinding := &rbacv1.RoleBinding{}
	err = scheme.Scheme.Convert(unstructuredResp, newRoleBinding, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newRoleBinding, nil
}
