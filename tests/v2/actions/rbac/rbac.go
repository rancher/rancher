package rbac

import (
	"fmt"
	"strings"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
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
