package rbac

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"

	apiV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coreV1 "k8s.io/api/core/v1"
)

const (
	roleOwner                     = "cluster-owner"
	roleMember                    = "cluster-member"
	roleProjectOwner              = "project-owner"
	roleProjectMember             = "project-member"
	roleCustomManageProjectMember = "projectroletemplatebindings-manage"
	roleCustomCreateNS            = "create-ns"
	roleProjectReadOnly           = "read-only"
	restrictedAdmin               = "restricted-admin"
	standardUser                  = "user"
	activeStatus                  = "active"
	forbiddenError                = "403 Forbidden"
)

var rgx = regexp.MustCompile(`\[(.*?)\]`)

// VerifyGlobalRoleBindingsForUser validates that a global role bindings is created for a user when the user is created
func VerifyGlobalRoleBindingsForUser(t *testing.T, user *management.User, adminClient *rancher.Client) {
	query := url.Values{"filter": {"userName=" + user.ID}}
	grbs, err := adminClient.Steve.SteveType("management.cattle.io.globalrolebinding").List(query)
	require.NoError(t, err)
	assert.Equal(t, 1, len(grbs.Data))
}

// VerifyUserCanListCluster validates a user with the required global permissions are able to/not able to list the clusters in rancher server
func VerifyUserCanListCluster(t *testing.T, client, standardClient *rancher.Client, clusterID, role string) {
	clusterList, err := standardClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
	require.NoError(t, err)

	clusterStatus := &apiV1.ClusterStatus{}
	err = v1.ConvertToK8sType(clusterList.Data[0].Status, clusterStatus)
	require.NoError(t, err)

	if role == restrictedAdmin {
		adminClusterList, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
		require.NoError(t, err)
		assert.Equal(t, (len(adminClusterList.Data) - 1), len(clusterList.Data))
	}
	assert.Equal(t, 1, len(clusterList.Data))
	actualClusterID := clusterStatus.ClusterName
	assert.Equal(t, clusterID, actualClusterID)

}

// VerifyUserCanListProject validates a user with the required cluster permissions are able/not able to list projects in the downstream cluster
func VerifyUserCanListProject(t *testing.T, client, standardClient *rancher.Client, clusterID, role, adminProjectName string) {
	projectListNonAdmin, err := projects.ListProjectNames(standardClient, clusterID)
	require.NoError(t, err)
	projectListAdmin, err := projects.ListProjectNames(client, clusterID)
	require.NoError(t, err)

	switch role {
	case roleOwner, restrictedAdmin:
		assert.Equal(t, len(projectListAdmin), len(projectListNonAdmin))
		assert.Equal(t, projectListAdmin, projectListNonAdmin)
	case roleMember:
		assert.Equal(t, 0, len(projectListNonAdmin))
	case roleProjectOwner, roleProjectMember:
		assert.Equal(t, 1, len(projectListNonAdmin))
		assert.Equal(t, adminProjectName, projectListNonAdmin[0])
	}
}

// VerifyUserCanCreateProjects validates a user with the required cluster permissions are able/not able to create projects in the downstream cluster
func VerifyUserCanCreateProjects(t *testing.T, client, standardClient *rancher.Client, clusterID, role string) {
	memberProject, err := standardClient.Management.Project.Create(projects.NewProjectConfig(clusterID))
	switch role {
	case roleOwner, roleMember, restrictedAdmin:
		require.NoError(t, err)
		log.Info("Created project as a ", role, " is ", memberProject.Name)
		actualStatus := fmt.Sprintf("%v", memberProject.State)
		assert.Equal(t, activeStatus, strings.ToLower(actualStatus))
	case roleProjectOwner, roleProjectMember:
		require.Error(t, err)
		errStatus := strings.Split(err.Error(), ".")[1]
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(t, forbiddenError, errorMsg[1])
	}
}

// VerifyUserCanCreateNamespace validates a user with the required cluster permissions are able/not able to create namespaces in the project they do not own
func VerifyUserCanCreateNamespace(t *testing.T, client, standardClient *rancher.Client, project *management.Project, clusterID, role string) {
	var checkErr error
	namespaceName := namegen.AppendRandomString("testns-")
	standardClient, err := standardClient.ReLogin()
	require.NoError(t, err)

	createdNamespace, checkErr := namespaces.CreateNamespace(standardClient, namespaceName, "{}", map[string]string{}, map[string]string{}, project)

	switch role {
	case roleOwner, roleProjectOwner, roleProjectMember, restrictedAdmin:
		require.NoError(t, checkErr)
		log.Info("Created a namespace as role ", role, createdNamespace.Name)
		assert.Equal(t, namespaceName, createdNamespace.Name)

		namespaceStatus := &coreV1.NamespaceStatus{}
		err = v1.ConvertToK8sType(createdNamespace.Status, namespaceStatus)
		require.NoError(t, err)
		actualStatus := fmt.Sprintf("%v", namespaceStatus.Phase)
		assert.Equal(t, activeStatus, strings.ToLower(actualStatus))
	case roleMember:
		require.Error(t, checkErr)
		errStatus := strings.Split(checkErr.Error(), ".")[1]
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(t, forbiddenError, errorMsg[1])
	}
}

// VerifyUserCanListNamespace validates a user with the required cluster permissions are able/not able to list namespaces in the project they do not own
func VerifyUserCanListNamespace(t *testing.T, client, standardClient *rancher.Client, project *management.Project, clusterID, role string) {
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
	case roleOwner, restrictedAdmin:
		require.NoError(t, err)
		assert.Equal(t, len(sortedNamespaceListAdmin), len(sortedNamespaceListNonAdmin))
		assert.Equal(t, sortedNamespaceListAdmin, sortedNamespaceListNonAdmin)
	case roleMember:
		require.NoError(t, err)
		assert.Equal(t, 0, len(sortedNamespaceListNonAdmin))
	case roleProjectOwner, roleProjectMember:
		require.NoError(t, err)
		assert.NotEqual(t, len(sortedNamespaceListAdmin), len(sortedNamespaceListNonAdmin))
		assert.Equal(t, 1, len(sortedNamespaceListNonAdmin))
	}
}

// VerifyUserCanDeleteNamespace validates a user with the required cluster permissions are able/not able to delete namespaces in the project they do not own
func VerifyUserCanDeleteNamespace(t *testing.T, client, standardClient *rancher.Client, project *management.Project, clusterID, role string) {

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
	case roleOwner, roleProjectOwner, roleProjectMember, restrictedAdmin:
		require.NoError(t, err)
	case roleMember:
		require.Error(t, err)
		errMessage := strings.Split(err.Error(), ":")[0]
		assert.Equal(t, "Resource type [namespace] can not be deleted", errMessage)
	}
}

// VerifyUserCanAddClusterRoles validates a user with the required cluster permissions are able/not able to add other users in the cluster
func VerifyUserCanAddClusterRoles(t *testing.T, client, memberClient *rancher.Client, cluster *management.Cluster, role string) {
	additionalClusterUser, err := users.CreateUserWithRole(client, users.UserConfig(), standardUser)
	require.NoError(t, err)

	errUserRole := users.AddClusterRoleToUser(memberClient, cluster, additionalClusterUser, roleOwner, nil)

	switch role {
	case roleProjectOwner, roleProjectMember:
		require.Error(t, errUserRole)
		errStatus := strings.Split(errUserRole.Error(), ".")[1]
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(t, forbiddenError, errorMsg[1])
	case restrictedAdmin:
		require.NoError(t, errUserRole)
	}

}

// VerifyUserCanAddProjectRoles validates a user with the required cluster permissions are able/not able to add other users in a project on the downstream cluster
func VerifyUserCanAddProjectRoles(t *testing.T, client *rancher.Client, project *management.Project, additionalUser *management.User, projectRole, clusterID, role string) {

	errUserRole := users.AddProjectMember(client, project, additionalUser, projectRole, nil)
	projectList, errProjectList := projects.ListProjectNames(client, clusterID)
	require.NoError(t, errProjectList)

	switch role {
	case roleProjectOwner:
		require.NoError(t, errUserRole)
		assert.Equal(t, 1, len(projectList))
		assert.Equal(t, project.Name, projectList[0])

	case restrictedAdmin:
		require.NoError(t, errUserRole)
		assert.Contains(t, projectList, project.Name)

	case roleProjectMember:
		require.Error(t, errUserRole)
	}

}

// VerifyUserCanDeleteProject validates a user with the required cluster/project permissions are able/not able to delete projects in the downstream cluster
func VerifyUserCanDeleteProject(t *testing.T, client *rancher.Client, project *management.Project, role string) {
	err := client.Management.Project.Delete(project)

	switch role {
	case roleOwner, roleProjectOwner:
		require.NoError(t, err)
	case roleMember:
		require.Error(t, err)
		errStatus := strings.Split(err.Error(), ".")[1]
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(t, forbiddenError, errorMsg[1])
	case roleProjectMember:
		require.Error(t, err)
	}
}

// VerifyUserCanRemoveClusterRoles validates a user with the required cluster/project permissions are able/not able to remove cluster roles in the downstream cluster
func VerifyUserCanRemoveClusterRoles(t *testing.T, client *rancher.Client, user *management.User) {
	err := users.RemoveClusterRoleFromUser(client, user)
	require.NoError(t, err)
}
