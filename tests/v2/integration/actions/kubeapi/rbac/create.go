package rbac

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateRole is a helper function that uses the dynamic client to create a role on a namespace for a specific cluster.
func CreateRole(client *rancher.Client, clusterName string, role *rbacv1.Role) (*rbacv1.Role, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	roleResource := dynamicClient.Resource(RoleGroupVersionResource).Namespace(role.Namespace)

	unstructuredResp, err := roleResource.Create(context.Background(), unstructured.MustToUnstructured(role), metav1.CreateOptions{})
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

// CreateRoleBinding is a helper function that uses the dynamic client to create a rolebinding on a namespace for a specific cluster.
func CreateRoleBinding(client *rancher.Client, clusterName, roleBindingName, namespace, roleName string, subject rbacv1.Subject) (*rbacv1.RoleBinding, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "Role",
			Name:     roleName,
		},
	}

	roleBindingResource := dynamicClient.Resource(RoleBindingGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := roleBindingResource.Create(context.Background(), unstructured.MustToUnstructured(roleBinding), metav1.CreateOptions{})
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

// CreateGlobalRole is a helper function that uses the dynamic client to create a global role in the local cluster.
func CreateGlobalRole(client *rancher.Client, globalRole *v3.GlobalRole) (*v3.GlobalRole, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	globalRoleResource := dynamicClient.Resource(GlobalRoleGroupVersionResource)
	unstructuredResp, err := globalRoleResource.Create(context.TODO(), unstructured.MustToUnstructured(globalRole), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newGlobalRole := &v3.GlobalRole{}
	err = scheme.Scheme.Convert(unstructuredResp, newGlobalRole, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newGlobalRole, nil
}

// CreateGlobalRoleBinding is a helper function that uses the dynamic client to create a global role binding for a specific user.
func CreateGlobalRoleBinding(client *rancher.Client, globalRoleBinding *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	globalRoleBindingResource := dynamicClient.Resource(GlobalRoleBindingGroupVersionResource)
	unstructuredResp, err := globalRoleBindingResource.Create(context.TODO(), unstructured.MustToUnstructured(globalRoleBinding), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newGlobalRoleBinding := &v3.GlobalRoleBinding{}
	err = scheme.Scheme.Convert(unstructuredResp, newGlobalRoleBinding, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newGlobalRoleBinding, nil
}

// CreateRoleTemplate is a helper function that uses the dynamic client to create a cluster/project role template for a specific user
func CreateRoleTemplate(client *rancher.Client, roleTemplate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	roleTemplateResource := dynamicClient.Resource(RoleTemplateGroupVersionResource)
	unstructuredResp, err := roleTemplateResource.Create(context.Background(), unstructured.MustToUnstructured(roleTemplate), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newRoleTemplate := &v3.RoleTemplate{}
	err = scheme.Scheme.Convert(unstructuredResp, newRoleTemplate, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newRoleTemplate, nil
}

// CreateProjectRoleTemplateBinding is a helper function that uses the dynamic client to create a project role template binding in the local cluster.
func CreateProjectRoleTemplateBinding(client *rancher.Client, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	projectRoleTemplateBindingResource := dynamicClient.Resource(ProjectRoleTemplateBindingGroupVersionResource).Namespace(prtb.Namespace)
	unstructuredResp, err := projectRoleTemplateBindingResource.Create(context.TODO(), unstructured.MustToUnstructured(prtb), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newprtb := &v3.ProjectRoleTemplateBinding{}
	err = scheme.Scheme.Convert(unstructuredResp, newprtb, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newprtb, nil
}

// CreateClusterRoleTemplateBinding is a helper function that uses the dynamic client to create a cluster role template binding for a specific user
// in the given downstream cluster.
func CreateClusterRoleTemplateBinding(client *rancher.Client, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	clusterRoleTemplateBindingResource := dynamicClient.Resource(ClusterRoleTemplateBindingGroupVersionResource).Namespace(crtb.Namespace)
	unstructuredResp, err := clusterRoleTemplateBindingResource.Create(context.Background(), unstructured.MustToUnstructured(crtb), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newClusterRoleTemplateBinding := &v3.ClusterRoleTemplateBinding{}
	err = scheme.Scheme.Convert(unstructuredResp, newClusterRoleTemplateBinding, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newClusterRoleTemplateBinding, nil
}
