package rbac

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListRoleBindings is a helper function that uses the dynamic client to list rolebindings on a namespace for a specific cluster.
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
func ListGlobalRoleBindings(client *rancher.Client, listOpt metav1.ListOptions) (*v3.GlobalRoleBindingList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(GlobalRoleBindingGroupVersionResource).List(context.TODO(), listOpt)
	if err != nil {
		return nil, err
	}

	grbList := new(v3.GlobalRoleBindingList)
	for _, unstructuredGRB := range unstructuredList.Items {
		grb := &v3.GlobalRoleBinding{}
		err := scheme.Scheme.Convert(&unstructuredGRB, grb, unstructuredGRB.GroupVersionKind())
		if err != nil {
			return nil, err
		}
		grbList.Items = append(grbList.Items, *grb)
	}

	return grbList, nil
}

// ListClusterRoleTemplateBindings is a helper function that uses the dynamic client to list clusterroletemplatebindings from local cluster.
func ListClusterRoleTemplateBindings(client *rancher.Client, listOpt metav1.ListOptions) (*v3.ClusterRoleTemplateBindingList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(ClusterRoleTemplateBindingGroupVersionResource).Namespace("").List(context.TODO(), listOpt)
	if err != nil {
		return nil, err
	}

	crtbList := new(v3.ClusterRoleTemplateBindingList)
	for _, unstructuredCRTB := range unstructuredList.Items {
		crtb := &v3.ClusterRoleTemplateBinding{}
		err := scheme.Scheme.Convert(&unstructuredCRTB, crtb, unstructuredCRTB.GroupVersionKind())
		if err != nil {
			return nil, err
		}
		crtbList.Items = append(crtbList.Items, *crtb)
	}

	return crtbList, nil
}

// ListGlobalRoles is a helper function that uses the dynamic client to list globalroles from local cluster.
func ListGlobalRoles(client *rancher.Client, listOpt metav1.ListOptions) (*v3.GlobalRoleList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(GlobalRoleGroupVersionResource).List(context.TODO(), listOpt)
	if err != nil {
		return nil, err
	}

	grList := new(v3.GlobalRoleList)
	for _, unstructuredGR := range unstructuredList.Items {
		gr := &v3.GlobalRole{}
		err := scheme.Scheme.Convert(&unstructuredGR, gr, unstructuredGR.GroupVersionKind())
		if err != nil {
			return nil, err
		}
		grList.Items = append(grList.Items, *gr)
	}

	return grList, nil
}

// ListRoleTemplates is a helper function that uses the dynamic client to list role templates from local cluster.
func ListRoleTemplates(client *rancher.Client, listOpt metav1.ListOptions) (*v3.RoleTemplateList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(RoleTemplateGroupVersionResource).List(context.TODO(), listOpt)
	if err != nil {
		return nil, err
	}

	rtList := new(v3.RoleTemplateList)
	for _, unstructuredRT := range unstructuredList.Items {
		rt := &v3.RoleTemplate{}
		err := scheme.Scheme.Convert(&unstructuredRT, rt, unstructuredRT.GroupVersionKind())
		if err != nil {
			return nil, err
		}
		rtList.Items = append(rtList.Items, *rt)
	}

	return rtList, nil
}
