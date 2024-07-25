package rbac

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/kubeapi/rbac"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRoleBindings(rancherClient *rancher.Client, clusterID string, userID string) ([]rbacv1.RoleBinding, error) {
	logrus.Infof("Getting role bindings for user %s in cluster %s", userID, clusterID)
	listOpt := v1.ListOptions{}
	roleBindings, err := rbac.ListRoleBindings(rancherClient, clusterID, "", listOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RoleBindings: %w", err)
	}

	var userRoleBindings []rbacv1.RoleBinding
	for _, rb := range roleBindings.Items {
		for _, subject := range rb.Subjects {
			if subject.Name == userID {
				userRoleBindings = append(userRoleBindings, rb)
				break
			}
		}
	}
	logrus.Infof("Found %d role bindings for user %s", len(userRoleBindings), userID)
	return userRoleBindings, nil
}

func GetBindings(rancherClient *rancher.Client, userID string) (map[string]interface{}, error) {
	logrus.Infof("Getting all bindings for user %s", userID)
	bindings := make(map[string]interface{})

	roleBindings, err := GetRoleBindings(rancherClient, rbac.LocalCluster, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role bindings: %w", err)
	}
	bindings["RoleBindings"] = roleBindings

	logrus.Info("Getting cluster role bindings")
	clusterRoleBindings, err := rbac.ListClusterRoleBindings(rancherClient, rbac.LocalCluster, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster role bindings: %w", err)
	}
	bindings["ClusterRoleBindings"] = clusterRoleBindings.Items

	logrus.Info("Getting global role bindings")
	globalRoleBindings, err := rancherClient.Management.GlobalRoleBinding.ListAll(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list global role bindings: %w", err)
	}
	bindings["GlobalRoleBindings"] = globalRoleBindings.Data

	logrus.Info("Getting cluster role template bindings")
	clusterRoleTemplateBindings, err := rancherClient.Management.ClusterRoleTemplateBinding.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster role template bindings: %w", err)
	}
	bindings["ClusterRoleTemplateBindings"] = clusterRoleTemplateBindings.Data

	logrus.Info("All bindings retrieved successfully")
	return bindings, nil
}
