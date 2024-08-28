package rbac

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteGlobalRoleBinding is a helper function that uses the dynamic client to delete a Global Role Binding by name
func DeleteGlobalRoleBinding(client *rancher.Client, globalRoleBindingName string) error {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return err
	}

	globalRoleBindingResource := dynamicClient.Resource(GlobalRoleBindingGroupVersionResource)

	err = globalRoleBindingResource.Delete(context.TODO(), globalRoleBindingName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// DeleteGlobalRole is a helper function that uses the dynamic client to delete a Global Role by name
func DeleteGlobalRole(client *rancher.Client, globalRoleName string) error {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return err
	}

	globalRoleResource := dynamicClient.Resource(GlobalRoleGroupVersionResource)

	err = globalRoleResource.Delete(context.TODO(), globalRoleName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// DeleteRoletemplate is a helper function that uses the dynamic client to delete a Custom Cluster Role/ Project Role template by name
func DeleteRoletemplate(client *rancher.Client, roleName string) error {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return err
	}

	roleResource := dynamicClient.Resource(RoleTemplateGroupVersionResource)

	err = roleResource.Delete(context.TODO(), roleName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
