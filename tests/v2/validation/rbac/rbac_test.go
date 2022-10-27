package rbac

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RBTestSuite struct {
	suite.Suite
	client                *rancher.Client
	standardUser          *management.User
	standardUserClient    *rancher.Client
	session               *session.Session
	cluster               *management.Cluster
	adminProject          *management.Project
	standardUserCOProject *management.Project
	additionalUser        *management.User
	additionalUserClient  *rancher.Client
}

func (rb *RBTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *RBTestSuite) SetupSuite() {
	testSession := session.NewSession(rb.T())
	rb.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(rb.T(), err)

	rb.client = client

	//Get cluster name from the config file and append cluster details in rb
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rb.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rb.client, clusterName)
	require.NoError(rb.T(), err, "Error getting cluster ID")
	rb.cluster, err = rb.client.Management.Cluster.ByID(clusterID)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) ValidateListCluster(role string) {

	//Testcase1 Verify cluster members - Owner/member are able to list clusters
	clusterList, err := listClusters(rb.standardUserClient)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))
	assert.Equal(rb.T(), rb.cluster.ID, clusterList.Data[0].Status.ClusterName)
}

func (rb *RBTestSuite) ValidateListProjects(role string) {

	//Testcase2 Verify members of cluster are able to list the projects in a cluster
	//Get project list as an admin
	projectlistAdmin, err := listProjects(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	//Get project list as a cluster owner/member
	projectlistClusterMembers, err := listProjects(rb.standardUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	switch role {
	case roleOwner:
		//assert length of projects list obtained as an admin and a cluster owner are equal
		assert.Equal(rb.T(), len(projectlistAdmin), len(projectlistClusterMembers))
		//assert projects values obtained as an admin and the cluster owner are the same
		assert.Equal(rb.T(), projectlistAdmin, projectlistClusterMembers)
	case roleMember:
		//assert projects list obtained as a cluster member is empty
		assert.Equal(rb.T(), 0, len(projectlistClusterMembers))
	}
}

func (rb *RBTestSuite) ValidateCreateProjects(role string) {

	//Testcase3 Validate if cluster members can create a project in the downstream cluster
	createProjectAsClusterMembers, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	log.Info("Created project as a ", role, " is ", createProjectAsClusterMembers.Name)
	require.NoError(rb.T(), err)
	actualStatus := fmt.Sprintf("%v", rb.adminProject.State)
	assert.Equal(rb.T(), "active", actualStatus)

}

func (rb *RBTestSuite) ValidateNS(role string) {

	//Testcase4 Validate if cluster members can create namespaces in project they are not owner of
	log.Info("Testcase4 - Validating if ", role, " can create namespace in a project they are not owner of. ")
	namespaceName := provisioning.AppendRandomString("testns-")
	createdNamespace, err := namespaces.CreateNamespace(rb.standardUserClient, namespaceName, "{}", map[string]string{}, map[string]string{}, rb.adminProject)
	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
		log.Info("Created a namespace as cluster Owner: ", createdNamespace.Name)
		assert.Equal(rb.T(), namespaceName, createdNamespace.Name)
		actualStatus := fmt.Sprintf("%v", createdNamespace.Status.Phase)
		assert.Equal(rb.T(), "Active", actualStatus)
	case roleMember:
		require.Error(rb.T(), err)
		//assert cluster member gets an error when creating a namespace in a project they are not owner of
		errMessage := strings.Split(err.Error(), ":")[0]
		assert.Equal(rb.T(), "namespaces is forbidden", errMessage)
	}

	//Testcase5 Validate if cluster members are able to list all the namespaces in a cluster
	log.Info("Testcase5 - Validating if ", role, " can lists all namespaces in a cluster.")
	//Get the list of namespaces as and admin client
	namespaceListAdmin, err := getNamespaces(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	//Get the list of namespaces as an admin client
	namespaceListClusterMembers, err := getNamespaces(rb.standardUserClient, rb.cluster.ID)

	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
		//Length of namespace list for admin and cluster owner should match
		assert.Equal(rb.T(), len(namespaceListAdmin), len(namespaceListClusterMembers))
		//Namespaces obtained as admin and cluster owner should be same
		assert.Equal(rb.T(), namespaceListAdmin, namespaceListClusterMembers)
	case roleMember:
		require.Error(rb.T(), err)
		errMessage := strings.Split(err.Error(), ":")[0]
		//assert cluster member is not able to list namespaces
		assert.Equal(rb.T(), "namespaces is forbidden", errMessage)
	}

	//Testcase6 Validate if cluster members are able to delete the namespace in the project they are not owner of
	log.Info("Testcase6 - Validating if ", role, " can delete a namespace from a project they are not owner of.")
	err = namespaces.DeleteNamespace(rb.standardUserClient, namespaceName, rb.cluster.ID)
	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
	case roleMember:
		require.Error(rb.T(), err)
	}
}

func (rb *RBTestSuite) ValidateDeleteProject(role string) {

	//Testcase7 Validate if cluster members are able to delete the project they are not owner of
	err := rb.standardUserClient.Management.Project.Delete(rb.adminProject)

	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
	case roleMember:
		require.Error(rb.T(), err)
		errStatus := strings.Split(err.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
	}
}

func (rb *RBTestSuite) ValidateRemoveClusterRoles(role string) {

	//Testcase8 Remove added cluster member from the cluster as an admin
	err := users.RemoveClusterRoleFromUser(rb.client, rb.standardUser)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) TestRBAC() {
	tests := []struct {
		name        string
		clusterRole string
	}{
		{"Cluster Owner", roleOwner},
		{"Cluster Member", roleMember},
	}
	for _, tt := range tests {
		rb.Run("Set up User with Cluster Role "+tt.name, func() {
			newUser, err := createUser(rb.client)
			require.NoError(rb.T(), err)
			rb.standardUser = newUser
			rb.T().Logf("Created user: %v", rb.standardUser.Username)
			rb.standardUserClient, err = rb.client.AsUser(newUser)
			require.NoError(rb.T(), err)

			subSession := rb.session.NewSession()
			defer subSession.Cleanup()

			createProjectAsAdmin, err := createProject(rb.client, rb.cluster.ID)
			rb.adminProject = createProjectAsAdmin
			require.NoError(rb.T(), err)
		})

		//Verify standard users cannot list any clusters
		rb.Run("Test case Validate standard users cannot list any downstream clusters before adding the cluster role "+tt.name, func() {
			_, err := listClusters(rb.standardUserClient)
			require.Error(rb.T(), err)
			assert.Equal(rb.T(), "Resource type [provisioning.cattle.io.cluster] is not listable", err.Error())
		})

		rb.Run("Adding user as "+tt.name+" to the downstream cluster.", func() {
			//Adding created user to the downstream clusters with the specified roles.
			err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, tt.clusterRole)
			require.NoError(rb.T(), err)
			rb.standardUserClient, err = rb.standardUserClient.ReLogin()
			require.NoError(rb.T(), err)
		})

		rb.T().Logf("Starting validations for %v", tt.clusterRole)

		rb.Run("Testcase1 - Validating the cluster count obtained as the role "+tt.name, func() {
			rb.ValidateListCluster(tt.clusterRole)
		})

		rb.Run("Testcase2 - Validating if members with role "+tt.name+" are able to list all projects", func() {
			rb.ValidateListProjects(tt.clusterRole)
		})

		rb.Run("Testcase3 - Validating if members with role "+tt.name+" is able to create a project in the cluster", func() {
			rb.ValidateCreateProjects(tt.clusterRole)

		})

		rb.Run("Testcase 4 through 6 - Validate namespaces checks for members with role "+tt.name, func() {
			rb.ValidateNS(tt.clusterRole)

		})
		rb.Run("Testcase7 - Validating if member with role "+tt.name+" can delete a project they are not owner of ", func() {
			rb.ValidateDeleteProject(tt.clusterRole)
		})

		rb.Run("Testcase8 - Validating if member with role "+tt.name+" is removed from the cluster and returns nil clusters", func() {
			rb.ValidateRemoveClusterRoles(tt.clusterRole)
		})

	}
}

func (rb *RBTestSuite) ValidateAddCMAsProjectOwner(role string) {

	//Additional test1 Add members

	errUserRole := users.AddClusterRoleToUser(rb.standardUserClient, rb.cluster, rb.additionalUser, roleMember)
	require.NoError(rb.T(), errUserRole)
	additionalUserClient, err := rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.additionalUserClient = additionalUserClient

	clusterList, err := listClusters(rb.additionalUserClient)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))

	//Add the cluster member to the project created as by the cluster Owner
	err = users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, "project-owner")
	require.NoError(rb.T(), err)
	userGetProject, err := projects.GetProjectList(rb.additionalUserClient, rb.cluster.ID)
	assert.Equal(rb.T(), rb.standardUserCOProject.Name, userGetProject.Data[0].Name)

}

func (rb *RBTestSuite) ValidateAddStdUserAsProjectOwner(role string) {

	//Create project as a cluster Owner
	createProjectAsCO, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.standardUserCOProject = createProjectAsCO

	//Additional test2 Validate if cluster members can create a project in the downstream cluster
	err = users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, "project-owner")
	require.NoError(rb.T(), err)
	userGetProject, err := projects.GetProjectList(rb.additionalUserClient, rb.cluster.ID)
	assert.Equal(rb.T(), rb.standardUserCOProject.Name, userGetProject.Data[0].Name)

	err = users.RemoveProjectMember(rb.standardUserClient, rb.additionalUser)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) ValidateAddMemberAsClusterRoles(role string) {

	//Additional test3 Add members

	errUserRole := users.AddClusterRoleToUser(rb.standardUserClient, rb.cluster, rb.additionalUser, roleOwner)
	require.NoError(rb.T(), errUserRole)
	additionalUserClient, err := rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.additionalUserClient = additionalUserClient

	clusterList, err := listClusters(rb.additionalUserClient)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))

	err = users.RemoveClusterRoleFromUser(rb.standardUserClient, rb.additionalUser)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) TestRBACAdditional() {
	rb.Run("Set up User with Cluster Role "+roleOwner, func() {
		newUser, err := createUser(rb.client)
		require.NoError(rb.T(), err)
		rb.standardUser = newUser
		rb.T().Logf("Created user: %v", rb.standardUser.Username)
		rb.standardUserClient, err = rb.client.AsUser(newUser)
		require.NoError(rb.T(), err)

		subSession := rb.session.NewSession()
		defer subSession.Cleanup()

		rb.T().Logf("Adding user as " + roleOwner + " to the downstream cluster.")
		//Adding created user to the downstream clusters with the role cluster Owner.
		err = users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, roleOwner)
		require.NoError(rb.T(), err)
		rb.standardUserClient, err = rb.standardUserClient.ReLogin()
		require.NoError(rb.T(), err)

		//Setting up an additional user for the additional rbac cases
		additionalUser, err := createUser(rb.client)
		require.NoError(rb.T(), err)
		rb.additionalUser = additionalUser
		rb.additionalUserClient, err = rb.client.AsUser(rb.additionalUser)
		require.NoError(rb.T(), err)
	})

	rb.Run("Additional test1 - Validating if member with role "+roleOwner+" can add another standard user as a project owner", func() {
		rb.ValidateAddStdUserAsProjectOwner(roleOwner)

	})

	rb.Run("Additional test2 - Validating if member with role "+roleOwner+" can add another standard user as a cluster owner", func() {
		rb.ValidateAddMemberAsClusterRoles(roleOwner)

	})

	rb.Run("Additional test3 - Validating if member with role "+roleOwner+" can add a cluster member as a project owner", func() {
		rb.ValidateAddCMAsProjectOwner(roleOwner)

	})

}

func TestRBACTestSuite(t *testing.T) {
	suite.Run(t, new(RBTestSuite))
}
