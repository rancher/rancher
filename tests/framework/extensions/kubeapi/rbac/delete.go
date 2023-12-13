package rbac

import (
	"context"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteGlobalRoleBinding is a helper function that uses the dynamic client to delete a Global Role Binding by name
func DeleteGlobalRoleBinding(client *rancher.Client, globalRoleBindingName string) error {
	dynamicClient, err := client.GetDownStreamClusterClient(localcluster)
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
	dynamicClient, err := client.GetDownStreamClusterClient(localcluster)
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
