package rbac

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rbacv2 "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

type Role string

const (
	Admin                     Role = "admin"
	RestrictedAdmin           Role = "restricted-admin"
	StandardUser              Role = "user"
	ClusterOwner              Role = "cluster-owner"
	ClusterMember             Role = "cluster-member"
	ProjectOwner              Role = "project-owner"
	ProjectMember             Role = "project-member"
	CreateNS                  Role = "create-ns"
	ReadOnly                  Role = "read-only"
	CustomManageProjectMember Role = "projectroletemplatebindings-manage"
	CrtbView                  Role = "clusterroletemplatebindings-view"
	ProjectsCreate            Role = "projects-create"
	ProjectsView              Role = "projects-view"
	ActiveStatus                   = "active"
	ForbiddenError                 = "403 Forbidden"
	DefaultNamespace               = "fleet-default"
	LocalCluster                   = "local"
	UserKind                       = "User"
)

func (r Role) String() string {
	return string(r)
}

// AddUserWithRoleToCluster creates a user based on the global role and then adds the user to cluster with provided permissions.
func AddUserWithRoleToCluster(adminClient *rancher.Client, globalRole, role string, cluster *management.Cluster, project *management.Project) (*management.User, *rancher.Client, error) {
	user, userClient, err := SetupUser(adminClient, globalRole)
	if err != nil {
		return nil, nil, err
	}

	if globalRole == StandardUser.String() {
		if strings.Contains(role, "project") || role == ReadOnly.String() {
			err := users.AddProjectMember(adminClient, project, user, role, nil)
			if err != nil {
				return nil, nil, err
			}
		} else {
			err := users.AddClusterRoleToUser(adminClient, cluster, user, role, nil)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return user, userClient, nil
}

// SetupUser is a helper to create a user with the specified global role and a client for the user.
func SetupUser(client *rancher.Client, globalRole string) (user *management.User, userClient *rancher.Client, err error) {
	user, err = users.CreateUserWithRole(client, users.UserConfig(), globalRole)
	if err != nil {
		return
	}
	userClient, err = client.AsUser(user)
	if err != nil {
		return
	}
	return
}

// GetRoleBindings is a helper function to fetch rolebindings for a user
func GetRoleBindings(rancherClient *rancher.Client, clusterID string, userID string) ([]rbacv1.RoleBinding, error) {
	logrus.Infof("Getting role bindings for user %s in cluster %s", userID, clusterID)
	listOpt := v1.ListOptions{}
	roleBindings, err := rbacv2.ListRoleBindings(rancherClient, clusterID, "", listOpt)
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

// GetBindings is a helper function to fetch bindings for a user
func GetBindings(rancherClient *rancher.Client, userID string) (map[string]interface{}, error) {
	logrus.Infof("Getting all bindings for user %s", userID)
	bindings := make(map[string]interface{})

	roleBindings, err := GetRoleBindings(rancherClient, rbacv2.LocalCluster, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role bindings: %w", err)
	}
	bindings["RoleBindings"] = roleBindings

	logrus.Info("Getting cluster role bindings")
	clusterRoleBindings, err := rbacv2.ListClusterRoleBindings(rancherClient, rbacv2.LocalCluster, v1.ListOptions{})
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

// GetGlobalRoleBindingByUserAndRole is a helper function to fetch global role binding for a user associated with a specific global role
func GetGlobalRoleBindingByUserAndRole(client *rancher.Client, userID, globalRoleName string) (*v3.GlobalRoleBinding, error) {
	var matchingGlobalRoleBinding *v3.GlobalRoleBinding

	err := kwait.PollUntilContextTimeout(context.TODO(), defaults.TenSecondTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (done bool, pollErr error) {
		grblist, err := client.WranglerContext.Mgmt.GlobalRoleBinding().List(v1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, grb := range grblist.Items {
			if grb.GlobalRoleName == globalRoleName && grb.UserName == userID {
				matchingGlobalRoleBinding = &grb
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error while polling for global role binding: %w", err)
	}

	return matchingGlobalRoleBinding, nil
}

// GetGlobalRoleByName is a helper function to fetch global role by name
func GetGlobalRoleByName(client *rancher.Client, globalRoleName string) (*v3.GlobalRole, error) {
	var matchingGlobalRole *v3.GlobalRole

	err := kwait.PollUntilContextTimeout(context.TODO(), defaults.FiveSecondTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (done bool, pollErr error) {
		grlist, err := client.WranglerContext.Mgmt.GlobalRole().List(v1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, gr := range grlist.Items {
			if gr.Name == globalRoleName {
				matchingGlobalRole = &gr
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error while polling for global role: %w", err)
	}

	return matchingGlobalRole, nil
}

// GetGlobalRoleBindingByName is a helper function to fetch global role binding by name
func GetGlobalRoleBindingByName(client *rancher.Client, globalRoleBindingName string) (*v3.GlobalRoleBinding, error) {
	var matchingGlobalRoleBinding *v3.GlobalRoleBinding

	err := kwait.PollUntilContextTimeout(context.TODO(), defaults.FiveSecondTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (done bool, pollErr error) {
		grblist, err := client.WranglerContext.Mgmt.GlobalRoleBinding().List(v1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, grb := range grblist.Items {
			if grb.Name == globalRoleBindingName {
				matchingGlobalRoleBinding = &grb
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error while polling for global role binding: %w", err)
	}

	return matchingGlobalRoleBinding, nil
}
