package rbac

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	rbacv2 "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Role string

const (
	Admin                     Role = "admin"
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
	ManageWorkloads           Role = "workloads-manage"
	ActiveStatus                   = "active"
	ForbiddenError                 = "403 Forbidden"
	DefaultNamespace               = "fleet-default"
	LocalCluster                   = "local"
	UserKind                       = "User"
	ImageName                      = "nginx"
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

// VerifyRoleRules checks if the expected role rules match the actual rules.
func VerifyRoleRules(expected, actual map[string][]string) error {
	for resource, expectedVerbs := range expected {
		actualVerbs, exists := actual[resource]
		if !exists {
			return fmt.Errorf("resource %s not found in role rules", resource)
		}

		expectedSet := make(map[string]struct{})
		for _, verb := range expectedVerbs {
			expectedSet[verb] = struct{}{}
		}

		for _, verb := range actualVerbs {
			if _, found := expectedSet[verb]; !found {
				return fmt.Errorf("verbs for resource %s do not match: expected %v, got %v", resource, expectedVerbs, actualVerbs)
			}
		}
	}
	return nil
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
