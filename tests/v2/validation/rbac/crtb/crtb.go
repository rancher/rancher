package crtb

import (

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetClusterRoleTemplateBindings fetches cluster role template bindings for a specific user
func GetClusterRoleTemplateBindings(rancherClient *rancher.Client, userID string) ([]v3.ClusterRoleTemplateBinding, error) {
	listOpt := v1.ListOptions{}
	crtbList, err := rbac.ListClusterRoleTemplateBindings(rancherClient, listOpt)
	if err != nil {
		return nil, err
	}

	var userCRTBList []v3.ClusterRoleTemplateBinding
	for _, crtb := range crtbList.Items {
		if crtb.UserName == userID {
			userCRTBList = append(userCRTBList, crtb)
		}
	}

	return userCRTBList, nil
}