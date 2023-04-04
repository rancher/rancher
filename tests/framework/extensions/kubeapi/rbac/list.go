package rbac

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
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
