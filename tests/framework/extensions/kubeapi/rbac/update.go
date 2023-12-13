package rbac

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateGlobalRole is a helper function that uses the dynamic client to update a Global Role
func UpdateGlobalRole(client *rancher.Client, updatedGlobalRole *v3.GlobalRole) (*v3.GlobalRole, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(localcluster)
	if err != nil {
		return nil, err
	}
	globalRoleResource := dynamicClient.Resource(GlobalRoleGroupVersionResource)
	globalRolesUnstructured, err := globalRoleResource.Get(context.TODO(), updatedGlobalRole.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	currentGlobalRole := &v3.GlobalRole{}
	err = scheme.Scheme.Convert(globalRolesUnstructured, currentGlobalRole, globalRolesUnstructured.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	updatedGlobalRole.ObjectMeta.ResourceVersion = currentGlobalRole.ObjectMeta.ResourceVersion

	unstructuredResp, err := globalRoleResource.Update(context.TODO(), unstructured.MustToUnstructured(updatedGlobalRole), metav1.UpdateOptions{})
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
