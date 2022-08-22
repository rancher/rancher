package rbac

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
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
	client             *rancher.Client
	standardUser       *management.User
	standardUserClient *rancher.Client
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
	clusterList, err := rb.standardUserClient.Steve.SteveType(clusters.ProvisioningSteveResouceType).ListAll(&types.ListOpts{})
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))
	actualClusterID := clusterList.Data[0].Status.(interface{}).(map[string]interface{})["clusterName"]
	assert.Equal(rb.T(), rb.cluster.ID, actualClusterID)
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
	createProjectAsClusterMembers, err := createProject(rb.standardUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	log.Info("Created project as a ", role, " is ", createProjectAsClusterMembers.Name)
	require.NoError(rb.T(), err)
	actualStatus := fmt.Sprintf("%v", createProjectAsClusterMembers.State)
	assert.Equal(rb.T(), "active", actualStatus)

}

func (rb *RBTestSuite) ValidateNS(role string) {

	//Testcase4 Validate if cluster members can create namespaces in project they are not owner of
	log.Info("Testcase4 - Validating if ", role, " can create namespace in a project they are not owner of. ")
	namespaceName := provisioning.AppendRandomString("testns-")
	createdNamespace, err := namespaces.CreateNamespace(rb.standardUserClient, namespaceName, "{}", map[string]string{}, map[string]string{}, rb.adminProject)
	adminNamespace, err := namespaces.CreateNamespace(rb.client, namespaceName+"-admin", "{}", map[string]string{}, map[string]string{}, rb.adminProject)

	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
		log.Info("Created a namespace as cluster Owner: ", createdNamespace.Name)
		assert.Equal(rb.T(), namespaceName, createdNamespace.Name)
		actualStatus := fmt.Sprintf("%v", createdNamespace.Status.(interface{}).(map[string]interface{})["phase"])
		assert.Equal(rb.T(), "Active", actualStatus)
	case roleMember:
		require.Error(rb.T(), err)
		//assert cluster member gets an error when creating a namespace in a project they are not owner of
		errMessage := strings.Split(err.Error(), ":")[0]
		assert.Equal(rb.T(), "Resource type [namespace] is not creatable", errMessage)
	}

	//Testcase5 Validate if cluster members are able to list all the namespaces in a cluster
	log.Info("Testcase5 - Validating if ", role, " can lists all namespaces in a cluster.")

	//Get the list of namespaces as an admin client
	namespaceListAdmin, err := getNamespaces(rb.steveAdminClient)
	require.NoError(rb.T(), err)
	//Get the list of namespaces as an admin client
	namespaceListClusterMembers, err := getNamespaces(rb.steveStdUserclient)

	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
		//Length of namespace list for admin and cluster owner should match
		assert.Equal(rb.T(), len(namespaceListAdmin), len(namespaceListClusterMembers))
		//Namespaces obtained as admin and cluster owner should be same
		assert.Equal(rb.T(), namespaceListAdmin, namespaceListClusterMembers)
	case roleMember:
		require.NoError(rb.T(), err)
		//Length of namespace list cluster member should be nill
		assert.Equal(rb.T(), 0, len(namespaceListClusterMembers))
	}

	//Testcase6 Validate if cluster members are able to delete the namespace in the project they are not owner of
	log.Info("Testcase6 - Validating if ", role, " can delete a namespace from a project they are not owner of.")

	namespaceID, err := rb.steveAdminClient.SteveType(namespaces.NamespaceSteveType).ByID(adminNamespace.ID)
	err = deleteNamespace(namespaceID, rb.steveStdUserclient)
	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
	case roleMember:
		require.Error(rb.T(), err)
		errMessage := strings.Split(err.Error(), ":")[0]
		assert.Equal(rb.T(), "Resource type [namespace] can not be deleted", errMessage)
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

			steveAdminClient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
			require.NoError(rb.T(), err)
			rb.steveAdminClient = steveAdminClient

		})

		//Verify standard users cannot list any clusters
		rb.Run("Test case Validate standard users cannot list any downstream clusters before adding the cluster role "+tt.name, func() {
			_, err := rb.standardUserClient.Steve.SteveType(clusters.ProvisioningSteveResouceType).ListAll(&types.ListOpts{})
			require.Error(rb.T(), err)
			assert.Equal(rb.T(), "Resource type [provisioning.cattle.io.cluster] is not listable", err.Error())
		})

		rb.Run("Adding user as "+tt.name+" to the downstream cluster.", func() {
			//Adding created user to the downstream clusters with the specified roles.
			err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, tt.clusterRole)
			require.NoError(rb.T(), err)
			rb.standardUserClient, err = rb.standardUserClient.ReLogin()
			require.NoError(rb.T(), err)

			//Create a steve user client for a standard user to get the cluster details
			steveStdUserclient, err := rb.standardUserClient.Steve.ProxyDownstream(rb.cluster.ID)
			require.NoError(rb.T(), err)
			rb.steveStdUserclient = steveStdUserclient
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

func TestRBACTestSuite(t *testing.T) {
	suite.Run(t, new(RBTestSuite))
}
