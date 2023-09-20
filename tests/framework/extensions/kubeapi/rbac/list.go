package rbac

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListRoleBindings is a helper function that uses the dynamic client to list rolebindings on a namespace for a specific cluster.
// ListRoleBindings accepts ListOptions for specifying desired parameters for listed objects.
func ListRoleBindings(client *rancher.Client, clusterName, namespace string, listOpt metav1.ListOptions) (*rbacv1.RoleBindingList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(RoleBindingGroupVersionResource).Namespace(namespace).List(context.Background(), listOpt)
	if err != nil {
		return nil, err
	}

	rbList := new(rbacv1.RoleBindingList)
	for _, unstructuredRB := range unstructuredList.Items {
		rb := &rbacv1.RoleBinding{}
		err := scheme.Scheme.Convert(&unstructuredRB, rb, unstructuredRB.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		rbList.Items = append(rbList.Items, *rb)
	}

	return rbList, nil
}

// ListClusterRoleBindings is a helper function that uses the dynamic client to list clusterrolebindings for a specific cluster.
// ListClusterRoleBindings accepts ListOptions for specifying desired parameters for listed objects.
func ListClusterRoleBindings(client *rancher.Client, clusterName string, listOpt metav1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(ClusterRoleBindingGroupVersionResource).Namespace("").List(context.Background(), listOpt)
	if err != nil {
		return nil, err
	}

	crbList := new(rbacv1.ClusterRoleBindingList)
	for _, unstructuredCRB := range unstructuredList.Items {
		crb := &rbacv1.ClusterRoleBinding{}
		err := scheme.Scheme.Convert(&unstructuredCRB, crb, unstructuredCRB.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		crbList.Items = append(crbList.Items, *crb)
	}

	return crbList, nil
}

// ListGlobalRoleBindings is a helper function that uses the dynamic client to list globalrolebindings from local cluster.
// ListGlobalRoleBindings accepts ListOptions for specifying desired parameters for listed objects.
func ListGlobalRoleBindings(client *rancher.Client, clusterName string, listOpt metav1.ListOptions) (*v3.GlobalRoleBindingList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(GlobalRoleBindingGroupVersionResource).List(context.Background(), listOpt)
	if err != nil {
		return nil, err
	}

	grbList := new(v3.GlobalRoleBindingList)
	for _, unstructuredCRB := range unstructuredList.Items {
		grb := &v3.GlobalRoleBinding{}
		err := scheme.Scheme.Convert(&unstructuredCRB, grb, unstructuredCRB.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		grbList.Items = append(grbList.Items, *grb)
	}

	return grbList, nil
}


// ListClusterRoleTemplateBindings is a helper function that uses the dynamic client to list clusterroletemplatebindings from local cluster.
// ListClusterRoleTemplateBindings accepts ListOptions for specifying desired parameters for listed objects.
func ListClusterRoleTemplateBindings(client *rancher.Client, clusterName string, listOpt metav1.ListOptions) (*v3.ClusterRoleTemplateBindingList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(ClusterRoleTemplateBindingGroupVersionResource).Namespace("").List(context.Background(), listOpt)
	if err != nil {
		return nil, err
	}

	crtbList := new(v3.ClusterRoleTemplateBindingList)
	for _, unstructuredCRB := range unstructuredList.Items {
		crtb := &v3.ClusterRoleTemplateBinding{}
		err := scheme.Scheme.Convert(&unstructuredCRB, crtb, unstructuredCRB.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		crtbList.Items = append(crtbList.Items, *crtb)
	}

	return crtbList, nil
}