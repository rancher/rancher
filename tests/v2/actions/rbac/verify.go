package rbac

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	apiV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rbac "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VerifyGlobalRoleBindingsForUser validates that a global role bindings is created for a user when the user is created
func VerifyGlobalRoleBindingsForUser(t *testing.T, user *management.User, adminClient *rancher.Client) {
	query := url.Values{"filter": {"userName=" + user.ID}}
	grbs, err := adminClient.Steve.SteveType("management.cattle.io.globalrolebinding").List(query)
	require.NoError(t, err)
	assert.Equal(t, 1, len(grbs.Data))
}

// VerifyRoleBindingsForUser validates that the corresponding role bindings are created for the user
func VerifyRoleBindingsForUser(t *testing.T, user *management.User, adminClient *rancher.Client, clusterID string, role Role) {
	rblist, err := rbac.ListRoleBindings(adminClient, LocalCluster, clusterID, metav1.ListOptions{})
	require.NoError(t, err)
	userID := user.Resource.ID
	userRoleBindings := []string{}

	for _, rb := range rblist.Items {
		if rb.Subjects[0].Kind == UserKind && rb.Subjects[0].Name == userID {
			userRoleBindings = append(userRoleBindings, rb.Name)
		}
	}

	switch role {
	case ClusterOwner, ClusterMember:
		assert.Equal(t, 1, len(userRoleBindings))
	case ProjectOwner, ProjectMember, RestrictedAdmin:
		assert.Equal(t, 2, len(userRoleBindings))
	}
}

// VerifyUserCanListCluster validates a user with the required global permissions are able to/not able to list the clusters in rancher server
func VerifyUserCanListCluster(t *testing.T, client, standardClient *rancher.Client, clusterID string, role Role) {
	clusterList, err := standardClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
	require.NoError(t, err)

	clusterStatus := &apiV1.ClusterStatus{}
	err = v1.ConvertToK8sType(clusterList.Data[0].Status, clusterStatus)
	require.NoError(t, err)

	if role == RestrictedAdmin {
		adminClusterList, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
		require.NoError(t, err)
		assert.Equal(t, (len(adminClusterList.Data) - 1), len(clusterList.Data))
	}
	assert.Equal(t, 1, len(clusterList.Data))
	actualClusterID := clusterStatus.ClusterName
	assert.Equal(t, clusterID, actualClusterID)

}

// VerifyUserCanListProject validates a user with the required cluster permissions are able/not able to list projects in the downstream cluster
func VerifyUserCanListProject(t *testing.T, client, standardClient *rancher.Client, clusterID, adminProjectName string, role Role) {
	projectListNonAdmin, err := projects.ListProjectNames(standardClient, clusterID)
	require.NoError(t, err)
	projectListAdmin, err := projects.ListProjectNames(client, clusterID)
	require.NoError(t, err)

	switch role {
	case ClusterOwner, RestrictedAdmin:
		assert.Equal(t, len(projectListAdmin), len(projectListNonAdmin))
		assert.Equal(t, projectListAdmin, projectListNonAdmin)
	case ClusterMember:
		assert.Equal(t, 0, len(projectListNonAdmin))
	case ProjectOwner, ProjectMember:
		assert.Equal(t, 1, len(projectListNonAdmin))
		assert.Equal(t, adminProjectName, projectListNonAdmin[0])
	}
}

// VerifyUserCanCreateProjects validates a user with the required cluster permissions are able/not able to create projects in the downstream cluster
func VerifyUserCanCreateProjects(t *testing.T, client, standardClient *rancher.Client, clusterID string, role Role) {
	memberProject, err := standardClient.Management.Project.Create(projects.NewProjectConfig(clusterID))
	switch role {
	case ClusterOwner, ClusterMember, RestrictedAdmin:
		require.NoError(t, err)
		log.Info("Created project as a ", role, " is ", memberProject.Name)
		actualStatus := fmt.Sprintf("%v", memberProject.State)
		assert.Equal(t, ActiveStatus, strings.ToLower(actualStatus))
	case ProjectOwner, ProjectMember:
		require.Error(t, err)
		assert.Contains(t, err.Error(), ForbiddenError)
	}
}

// VerifyUserCanCreateNamespace validates a user with the required cluster permissions are able/not able to create namespaces in the project they do not own
func VerifyUserCanCreateNamespace(t *testing.T, client, standardClient *rancher.Client, project *management.Project, clusterID string, role Role) {
	namespaceName := namegen.AppendRandomString("testns-")
	standardClient, err := standardClient.ReLogin()
	require.NoError(t, err)

	createdNamespace, checkErr := namespaces.CreateNamespace(standardClient, namespaceName, "{}", map[string]string{}, map[string]string{}, project)

	switch role {
	case ClusterOwner, ProjectOwner, ProjectMember, RestrictedAdmin:
		require.NoError(t, checkErr)
		log.Info("Created a namespace as role ", role, createdNamespace.Name)
		assert.Equal(t, namespaceName, createdNamespace.Name)

		namespaceStatus := &coreV1.NamespaceStatus{}
		err = v1.ConvertToK8sType(createdNamespace.Status, namespaceStatus)
		require.NoError(t, err)
		actualStatus := fmt.Sprintf("%v", namespaceStatus.Phase)
		assert.Equal(t, ActiveStatus, strings.ToLower(actualStatus))
	case ClusterMember:
		require.Error(t, checkErr)
		assert.Contains(t, checkErr.Error(), ForbiddenError)
	}
}

// VerifyUserCanListNamespace validates a user with the required cluster permissions are able/not able to list namespaces in the project they do not own
func VerifyUserCanListNamespace(t *testing.T, client, standardClient *rancher.Client, project *management.Project, clusterID string, role Role) {
	log.Info("Validating if ", role, " can lists all namespaces in a cluster.")

	steveAdminClient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)
	steveStandardClient, err := standardClient.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	namespaceListAdmin, err := steveAdminClient.SteveType(namespaces.NamespaceSteveType).List(nil)
	require.NoError(t, err)
	sortedNamespaceListAdmin := namespaceListAdmin.Names()

	namespaceListNonAdmin, err := steveStandardClient.SteveType(namespaces.NamespaceSteveType).List(nil)
	require.NoError(t, err)
	sortedNamespaceListNonAdmin := namespaceListNonAdmin.Names()

	switch role {
	case ClusterOwner, RestrictedAdmin:
		require.NoError(t, err)
		assert.Equal(t, len(sortedNamespaceListAdmin), len(sortedNamespaceListNonAdmin))
		assert.Equal(t, sortedNamespaceListAdmin, sortedNamespaceListNonAdmin)
	case ClusterMember:
		require.NoError(t, err)
		assert.Equal(t, 0, len(sortedNamespaceListNonAdmin))
	case ProjectOwner, ProjectMember:
		require.NoError(t, err)
		assert.NotEqual(t, len(sortedNamespaceListAdmin), len(sortedNamespaceListNonAdmin))
		assert.Equal(t, 1, len(sortedNamespaceListNonAdmin))
	}
}

// VerifyUserCanDeleteNamespace validates a user with the required cluster permissions are able/not able to delete namespaces in the project they do not own
func VerifyUserCanDeleteNamespace(t *testing.T, client, standardClient *rancher.Client, project *management.Project, clusterID string, role Role) {

	log.Info("Validating if ", role, " cannot delete a namespace from a project they own.")
	steveAdminClient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)
	steveStandardClient, err := standardClient.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	namespaceName := namegen.AppendRandomString("testns-")
	adminNamespace, err := namespaces.CreateNamespace(client, namespaceName+"-admin", "{}", map[string]string{}, map[string]string{}, project)
	require.NoError(t, err)

	namespaceID, err := steveAdminClient.SteveType(namespaces.NamespaceSteveType).ByID(adminNamespace.ID)
	require.NoError(t, err)
	err = steveStandardClient.SteveType(namespaces.NamespaceSteveType).Delete(namespaceID)

	switch role {
	case ClusterOwner, ProjectOwner, ProjectMember, RestrictedAdmin:
		require.NoError(t, err)
	case ClusterMember:
		require.Error(t, err)
		assert.Equal(t, err.Error(), "Resource type [namespace] can not be deleted")
	}
}

// VerifyUserCanAddClusterRoles validates a user with the required cluster permissions are able/not able to add other users in the cluster
func VerifyUserCanAddClusterRoles(t *testing.T, client, memberClient *rancher.Client, cluster *management.Cluster, role Role) {
	additionalClusterUser, err := users.CreateUserWithRole(client, users.UserConfig(), StandardUser.String())
	require.NoError(t, err)

	errUserRole := users.AddClusterRoleToUser(memberClient, cluster, additionalClusterUser, ClusterOwner.String(), nil)

	switch role {
	case ProjectOwner, ProjectMember:
		require.Error(t, errUserRole)
		assert.Contains(t, errUserRole.Error(), ForbiddenError)
	case RestrictedAdmin:
		require.NoError(t, errUserRole)
	}
}

// VerifyUserCanAddProjectRoles validates a user with the required cluster permissions are able/not able to add other users in a project on the downstream cluster
func VerifyUserCanAddProjectRoles(t *testing.T, client *rancher.Client, project *management.Project, additionalUser *management.User, projectRole, clusterID string, role Role) {

	errUserRole := users.AddProjectMember(client, project, additionalUser, projectRole, nil)
	projectList, errProjectList := projects.ListProjectNames(client, clusterID)
	require.NoError(t, errProjectList)

	switch role {
	case ProjectOwner:
		require.NoError(t, errUserRole)
		assert.Equal(t, 1, len(projectList))
		assert.Equal(t, project.Name, projectList[0])

	case RestrictedAdmin:
		require.NoError(t, errUserRole)
		assert.Contains(t, projectList, project.Name)

	case ProjectMember:
		require.Error(t, errUserRole)
	}

}

// VerifyUserCanDeleteProject validates a user with the required cluster/project permissions are able/not able to delete projects in the downstream cluster
func VerifyUserCanDeleteProject(t *testing.T, client *rancher.Client, project *management.Project, role Role) {
	err := client.Management.Project.Delete(project)

	switch role {
	case ClusterOwner, ProjectOwner:
		require.NoError(t, err)
	case ClusterMember:
		require.Error(t, err)
		assert.Contains(t, err.Error(), ForbiddenError)
	case ProjectMember:
		require.Error(t, err)
	}
}

// VerifyUserCanRemoveClusterRoles validates a user with the required cluster/project permissions are able/not able to remove cluster roles in the downstream cluster
func VerifyUserCanRemoveClusterRoles(t *testing.T, client *rancher.Client, user *management.User) {
	err := users.RemoveClusterRoleFromUser(client, user)
	require.NoError(t, err)
}
