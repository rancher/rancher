package rbac

import (
	"fmt"
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
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	coreV1 "k8s.io/api/core/v1"
)

const (
	roleOwner           = "cluster-owner"
	roleMember          = "cluster-member"
	roleProjectOwner    = "project-owner"
	roleProjectMember   = "project-member"
	roleProjectReadOnly = "read-only"
	restrictedAdmin     = "restricted-admin"
	standardUser        = "user"
)

type RBTestSuite struct {
	suite.Suite
	client             *rancher.Client
	nonAdminUser       *management.User
	nonAdminUserClient *rancher.Client
	session            *session.Session
	cluster            *management.Cluster
	adminProject       *management.Project
	steveAdminClient   *v1.Client
	steveStdUserclient *v1.Client
}

func (rb *RBTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *RBTestSuite) SetupSuite() {
	testSession := session.NewSession()
	rb.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(rb.T(), err)

	rb.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rb")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rb.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rb.client, clusterName)
	require.NoError(rb.T(), err, "Error getting cluster ID")
	rb.cluster, err = rb.client.Management.Cluster.ByID(clusterID)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) ValidateListCluster(role string) {

	clusterList, err := rb.nonAdminUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
	require.NoError(rb.T(), err)
	clusterStatus := &apiV1.ClusterStatus{}
	err = v1.ConvertToK8sType(clusterList.Data[0].Status, clusterStatus)
	require.NoError(rb.T(), err)

	if role == restrictedAdmin {
		adminClusterList, err := rb.client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
		require.NoError(rb.T(), err)
		assert.Equal(rb.T(), (len(adminClusterList.Data) - 1), len(clusterList.Data))
		return
	}
	assert.Equal(rb.T(), 1, len(clusterList.Data))
	actualClusterID := clusterStatus.ClusterName
	assert.Equal(rb.T(), rb.cluster.ID, actualClusterID)
}

func (rb *RBTestSuite) ValidateListProjects(role string) {

	//Get project list as an admin
	projectlistAdmin, err := projects.ListProjectNames(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	//Get project list as a cluster owner/member, project owner/member and restricted admin
	projectlistClusterMembers, err := projects.ListProjectNames(rb.nonAdminUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	switch role {
	case roleOwner, restrictedAdmin:
		assert.Equal(rb.T(), len(projectlistAdmin), len(projectlistClusterMembers))
		assert.Equal(rb.T(), projectlistAdmin, projectlistClusterMembers)
	case roleMember:
		assert.Equal(rb.T(), 0, len(projectlistClusterMembers))
	case roleProjectOwner, roleProjectMember:
		assert.Equal(rb.T(), 1, len(projectlistClusterMembers))
		assert.Equal(rb.T(), rb.adminProject.Name, projectlistClusterMembers[0])
	}
}

func (rb *RBTestSuite) ValidateCreateProjects(role string) {

	createProjectAsClusterMembers, err := rb.nonAdminUserClient.Management.Project.Create(projects.NewProjectConfig(rb.cluster.ID))
	switch role {
	case roleOwner, roleMember, restrictedAdmin:
		require.NoError(rb.T(), err)
		log.Info("Created project as a ", role, " is ", createProjectAsClusterMembers.Name)
		require.NoError(rb.T(), err)
		actualStatus := fmt.Sprintf("%v", createProjectAsClusterMembers.State)
		assert.Equal(rb.T(), "active", actualStatus)
	case roleProjectOwner, roleProjectMember:
		require.Error(rb.T(), err)
		errStatus := strings.Split(err.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
	}
}

func (rb *RBTestSuite) ValidateNS(role string) {
	var checkErr error

	log.Info("Validating if ", role, " can create namespace in a project they are not owner of. ")
	namespaceName := namegen.AppendRandomString("testns-")
	adminNamespace, err := namespaces.CreateNamespace(rb.client, namespaceName+"-admin", "{}", map[string]string{}, map[string]string{}, rb.adminProject)
	require.NoError(rb.T(), err)

	relogin, err := rb.nonAdminUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.nonAdminUserClient = relogin

	steveStdUserclient, err := rb.nonAdminUserClient.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveStdUserclient = steveStdUserclient

	createdNamespace, checkErr := namespaces.CreateNamespace(rb.nonAdminUserClient, namespaceName, "{}", map[string]string{}, map[string]string{}, rb.adminProject)

	switch role {
	case roleOwner, roleProjectOwner, roleProjectMember, restrictedAdmin:
		require.NoError(rb.T(), checkErr)
		log.Info("Created a namespace as role ", role, createdNamespace.Name)
		assert.Equal(rb.T(), namespaceName, createdNamespace.Name)

		namespaceStatus := &coreV1.NamespaceStatus{}
		err = v1.ConvertToK8sType(createdNamespace.Status, namespaceStatus)
		require.NoError(rb.T(), err)
		actualStatus := fmt.Sprintf("%v", namespaceStatus.Phase)
		assert.Equal(rb.T(), "Active", actualStatus)
	case roleMember:
		require.Error(rb.T(), checkErr)
		errStatus := strings.Split(checkErr.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
	}

	log.Info("Validating if ", role, " can lists all namespaces in a cluster.")

	namespaceListAdmin, err := namespaces.ListNamespaceNames(rb.steveAdminClient)
	require.NoError(rb.T(), err)
	namespaceListClusterMembers, err := namespaces.ListNamespaceNames(rb.steveStdUserclient)

	switch role {
	case roleOwner, restrictedAdmin:
		require.NoError(rb.T(), err)
		assert.Equal(rb.T(), len(namespaceListAdmin), len(namespaceListClusterMembers))
		assert.Equal(rb.T(), namespaceListAdmin, namespaceListClusterMembers)
	case roleMember:
		require.NoError(rb.T(), err)
		assert.Equal(rb.T(), 0, len(namespaceListClusterMembers))
	case roleProjectOwner, roleProjectMember:
		require.NoError(rb.T(), err)
		assert.NotEqual(rb.T(), len(namespaceListAdmin), len(namespaceListClusterMembers))
		assert.Equal(rb.T(), 2, len(namespaceListClusterMembers))
	}

	log.Info("Validating if ", role, " cannot delete a namespace from a project they own.")

	namespaceID, err := rb.steveAdminClient.SteveType(namespaces.NamespaceSteveType).ByID(adminNamespace.ID)
	require.NoError(rb.T(), err)
	err = rb.steveStdUserclient.SteveType(namespaces.NamespaceSteveType).Delete(namespaceID)
	switch role {
	case roleOwner, roleProjectOwner, roleProjectMember, restrictedAdmin:
		require.NoError(rb.T(), err)
	case roleMember:
		require.Error(rb.T(), err)
		errMessage := strings.Split(err.Error(), ":")[0]
		assert.Equal(rb.T(), "Resource type [namespace] can not be deleted", errMessage)
	}
}

func (rb *RBTestSuite) ValidateAddClusterRoles(role string) {
	additionalClusterUser, err := users.CreateUserWithRole(rb.client, users.UserConfig(), standardUser)
	require.NoError(rb.T(), err)

	errUserRole := users.AddClusterRoleToUser(rb.nonAdminUserClient, rb.cluster, additionalClusterUser, roleOwner)

	switch role {
	case roleProjectOwner, roleProjectMember:
		require.Error(rb.T(), errUserRole)
		errStatus := strings.Split(errUserRole.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
	case restrictedAdmin:
		require.NoError(rb.T(), errUserRole)

	}
}

func (rb *RBTestSuite) ValidateAddProjectRoles(role string) {

	additionalProjectUser, err := users.CreateUserWithRole(rb.client, users.UserConfig(), standardUser)
	require.NoError(rb.T(), err)
	additionalProjectUserClient, err := rb.client.AsUser(additionalProjectUser)
	require.NoError(rb.T(), err)

	errUserRole := users.AddProjectMember(rb.nonAdminUserClient, rb.adminProject, additionalProjectUser, roleProjectOwner)
	additionalProjectUserClient, err = additionalProjectUserClient.ReLogin()
	require.NoError(rb.T(), err)

	projectList, errProjectList := projects.ListProjectNames(additionalProjectUserClient, rb.cluster.ID)
	require.NoError(rb.T(), errProjectList)

	switch role {
	case roleProjectOwner, restrictedAdmin:
		require.NoError(rb.T(), errUserRole)
		require.NoError(rb.T(), errProjectList)
		assert.Equal(rb.T(), 1, len(projectList))
		assert.Equal(rb.T(), rb.adminProject.Name, projectList[0])
	case roleProjectMember:
		require.Error(rb.T(), errUserRole)
	}
}

func (rb *RBTestSuite) ValidateDeleteProject(role string) {

	err := rb.nonAdminUserClient.Management.Project.Delete(rb.adminProject)

	switch role {
	case roleOwner, roleProjectOwner:
		require.NoError(rb.T(), err)
	case roleMember:
		require.Error(rb.T(), err)
		errStatus := strings.Split(err.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
	case roleProjectMember:
		require.Error(rb.T(), err)
	}
}

func (rb *RBTestSuite) ValidateRemoveClusterRoles() {

	err := users.RemoveClusterRoleFromUser(rb.client, rb.nonAdminUser)
	require.NoError(rb.T(), err)
}

func (rb *RBTestSuite) ValidateRemoveProjectRoles() {

	err := users.RemoveProjectMember(rb.client, rb.nonAdminUser)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) TestRBAC() {
	tests := []struct {
		name   string
		role   string
		member string
	}{
		{"Cluster Owner", roleOwner, standardUser},
		{"Cluster Member", roleMember, standardUser},
		{"Project Owner", roleProjectOwner, standardUser},
		{"Project Member", roleProjectMember, standardUser},
		{"Restricted Admin", restrictedAdmin, restrictedAdmin},
	}
	for _, tt := range tests {
		rb.Run("Set up User with Cluster Role "+tt.name, func() {
			newUser, err := users.CreateUserWithRole(rb.client, users.UserConfig(), tt.member)
			require.NoError(rb.T(), err)
			rb.nonAdminUser = newUser
			rb.T().Logf("Created user: %v", rb.nonAdminUser.Username)
			rb.nonAdminUserClient, err = rb.client.AsUser(newUser)
			require.NoError(rb.T(), err)

			subSession := rb.session.NewSession()
			defer subSession.Cleanup()

			createProjectAsAdmin, err := rb.client.Management.Project.Create(projects.NewProjectConfig(rb.cluster.ID))
			rb.adminProject = createProjectAsAdmin
			require.NoError(rb.T(), err)

			steveAdminClient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
			require.NoError(rb.T(), err)
			rb.steveAdminClient = steveAdminClient

		})

		log.Info("Validating standard users cannot list any clusters")
		rb.Run("Test case Validate if users can list any downstream clusters before adding the cluster role "+tt.name, func() {
			_, err := rb.nonAdminUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
			if tt.member == standardUser {
				require.Error(rb.T(), err)
				assert.Contains(rb.T(), "Resource type [provisioning.cattle.io.cluster] has no method GET", err.Error())
			}
		})

		rb.Run("Adding user as "+tt.name+" to the downstream cluster.", func() {

			if tt.member == standardUser {
				if strings.Contains(tt.role, "project") {
					err := users.AddProjectMember(rb.client, rb.adminProject, rb.nonAdminUser, tt.role)
					require.NoError(rb.T(), err)
				} else {
					err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.nonAdminUser, tt.role)
					require.NoError(rb.T(), err)
				}
			}

			relogin, err := rb.nonAdminUserClient.ReLogin()
			require.NoError(rb.T(), err)
			rb.nonAdminUserClient = relogin

			//Create a steve user client for a standard user to get the cluster details
			steveStdUserclient, err := rb.nonAdminUserClient.Steve.ProxyDownstream(rb.cluster.ID)
			require.NoError(rb.T(), err)
			rb.steveStdUserclient = steveStdUserclient
		})

		rb.T().Logf("Starting validations for %v", tt.role)

		rb.Run("Validating the cluster count obtained as the role "+tt.name, func() {
			rb.ValidateListCluster(tt.role)
		})

		rb.Run("Validating if members with role "+tt.name+" are able to list all projects", func() {
			rb.ValidateListProjects(tt.role)
		})

		rb.Run("Validating if members with role "+tt.name+" is able to create a project in the cluster", func() {
			rb.ValidateCreateProjects(tt.role)

		})

		rb.Run("Validate namespaces checks for members with role "+tt.name, func() {
			rb.ValidateNS(tt.role)
		})

		if !strings.Contains(tt.role, "cluster") {
			rb.Run("Validating if member with role "+tt.name+" can add members to the cluster", func() {
				rb.ValidateAddClusterRoles(tt.role)
			})

			rb.Run("Validating if member with role "+tt.name+" can add members to the project", func() {
				rb.ValidateAddProjectRoles(tt.role)
			})
		}

		rb.Run("Validating if member with role "+tt.name+" can delete a project they are not owner of ", func() {
			rb.ValidateDeleteProject(tt.role)
		})

		rb.Run("Validating if member with role "+tt.name+" is removed from the cluster and returns nil clusters", func() {
			if tt.member == standardUser {
				if strings.Contains(tt.role, "project") {
					rb.ValidateRemoveProjectRoles()
				} else {
					rb.ValidateRemoveClusterRoles()
				}
			}
		})

	}
}

func TestRBACTestSuite(t *testing.T) {
	suite.Run(t, new(RBTestSuite))
}
