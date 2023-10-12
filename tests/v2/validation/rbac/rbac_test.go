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
	"github.com/rancher/rancher/tests/framework/extensions/users"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	coreV1 "k8s.io/api/core/v1"
)

type RBTestSuite struct {
	suite.Suite
	client               *rancher.Client
	standardUser         *management.User
	standardUserClient   *rancher.Client
	session              *session.Session
	cluster              *management.Cluster
	adminProject         *management.Project
	steveAdminClient     *v1.Client
	steveStdUserclient   *v1.Client
	additionalUser       *management.User
	additionalUserClient *rancher.Client
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

	clusterList, err := rb.standardUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
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
	projectlistAdmin, err := listProjects(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	//Get project list as a cluster owner/member, project owner/member and restricted admin
	projectlistClusterMembers, err := listProjects(rb.standardUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	switch role {
	case roleOwner, restrictedAdmin:
		//assert length of projects list obtained as an admin and a cluster owner are equal
		assert.Equal(rb.T(), len(projectlistAdmin), len(projectlistClusterMembers))
		//assert projects values obtained as an admin and the cluster owner are the same
		assert.Equal(rb.T(), projectlistAdmin, projectlistClusterMembers)
	case roleMember:
		//assert projects list obtained as a cluster member is empty
		assert.Equal(rb.T(), 0, len(projectlistClusterMembers))
	case roleProjectOwner, roleProjectMember:
		//assert projects list obtained as a project owner/member is 1
		assert.Equal(rb.T(), 1, len(projectlistClusterMembers))
		//assert project created by admin and project obtained by project owner is same
		assert.Equal(rb.T(), rb.adminProject.Name, projectlistClusterMembers[0])
	}
}

func (rb *RBTestSuite) ValidateCreateProjects(role string) {

	createProjectAsClusterMembers, err := createProject(rb.standardUserClient, rb.cluster.ID)
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

	relogin, err := rb.standardUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.standardUserClient = relogin

	steveStdUserclient, err := rb.standardUserClient.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveStdUserclient = steveStdUserclient

	createdNamespace, checkErr := namespaces.CreateNamespace(rb.standardUserClient, namespaceName, "{}", map[string]string{}, map[string]string{}, rb.adminProject)

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
		//assert cluster member gets an error when creating a namespace in a project they are not owner of
		errStatus := strings.Split(checkErr.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
	}

	//Validate if cluster members/project members are able to list all the namespaces in a cluster
	log.Info("Validating if ", role, " can lists all namespaces in a cluster.")

	//Get the list of namespaces as an admin client
	namespaceListAdmin, err := getNamespaces(rb.steveAdminClient)
	require.NoError(rb.T(), err)
	//Get the list of namespaces as an admin client
	namespaceListClusterMembers, err := getNamespaces(rb.steveStdUserclient)

	switch role {
	case roleOwner, restrictedAdmin:
		require.NoError(rb.T(), err)
		//Length of namespace list for admin and cluster owner should match
		assert.Equal(rb.T(), len(namespaceListAdmin), len(namespaceListClusterMembers))
		//Namespaces obtained as admin and cluster owner should be same
		assert.Equal(rb.T(), namespaceListAdmin, namespaceListClusterMembers)
	case roleMember:
		require.NoError(rb.T(), err)
		//Length of namespace list cluster member should be nill
		assert.Equal(rb.T(), 0, len(namespaceListClusterMembers))
	case roleProjectOwner, roleProjectMember:
		require.NoError(rb.T(), err)
		//Length of namespace list for admin and project owner/member should not match
		assert.NotEqual(rb.T(), len(namespaceListAdmin), len(namespaceListClusterMembers))
		assert.Equal(rb.T(), 2, len(namespaceListClusterMembers))
	}

	// Validate if cluster members are able to delete the namespace in the admin created project
	log.Info("Validating if ", role, " cannot delete a namespace from a project they own.")

	namespaceID, err := rb.steveAdminClient.SteveType(namespaces.NamespaceSteveType).ByID(adminNamespace.ID)
	require.NoError(rb.T(), err)
	err = deleteNamespace(namespaceID, rb.steveStdUserclient)
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

	errUserRole := users.AddClusterRoleToUser(rb.standardUserClient, rb.cluster, rb.additionalUser, roleOwner, nil)

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

	errUserRole := users.AddProjectMember(rb.standardUserClient, rb.adminProject, rb.additionalUser, roleProjectOwner, nil)

	additionalUserClient, err := rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.additionalUserClient = additionalUserClient

	projectList, errProjectList := listProjects(rb.additionalUserClient, rb.cluster.ID)
	require.NoError(rb.T(), errProjectList)

	switch role {
	case roleProjectOwner:
		require.NoError(rb.T(), errUserRole)
		assert.Equal(rb.T(), 1, len(projectList))
		assert.Equal(rb.T(), rb.adminProject.Name, projectList[0])

	case restrictedAdmin:
		require.NoError(rb.T(), errUserRole)
		assert.Contains(rb.T(), projectList, rb.adminProject.Name)

	case roleProjectMember:
		require.Error(rb.T(), errUserRole)
	}

}

func (rb *RBTestSuite) ValidateDeleteProject(role string) {

	err := rb.standardUserClient.Management.Project.Delete(rb.adminProject)

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

	err := users.RemoveClusterRoleFromUser(rb.client, rb.standardUser)
	require.NoError(rb.T(), err)
}

func (rb *RBTestSuite) ValidateRemoveProjectRoles() {

	err := users.RemoveProjectMember(rb.client, rb.standardUser)
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
			rb.standardUser = newUser
			rb.T().Logf("Created user: %v", rb.standardUser.Username)
			rb.standardUserClient, err = rb.client.AsUser(newUser)
			require.NoError(rb.T(), err)

			log.Info("Validating Global Role Binding is created for the user.")
			userId, err := users.GetUserIDByName(rb.client, rb.standardUser.Username)
			require.NoError(rb.T(), err)
			query := url.Values{"filter": {"userName=" + userId}}
			grbs, err := rb.client.Steve.SteveType("management.cattle.io.globalrolebinding").List(query)
			require.NoError(rb.T(), err)
			assert.Equal(rb.T(), 1, len(grbs.Data))

			subSession := rb.session.NewSession()
			defer subSession.Cleanup()

			createProjectAsAdmin, err := createProject(rb.client, rb.cluster.ID)
			rb.adminProject = createProjectAsAdmin
			require.NoError(rb.T(), err)

			steveAdminClient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
			require.NoError(rb.T(), err)
			rb.steveAdminClient = steveAdminClient
		})

		log.Info("Validating standard users cannot list any clusters")
		rb.Run("Test case Validate if users can list any downstream clusters before adding the cluster role "+tt.name, func() {
			_, err := rb.standardUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
			if tt.member == standardUser {
				require.Error(rb.T(), err)
				assert.Contains(rb.T(), "Resource type [provisioning.cattle.io.cluster] has no method GET", err.Error())
			}
		})

		rb.Run("Adding user as "+tt.name+" to the downstream cluster.", func() {
			log.Info("Adding created user to the downstream clusters with the specified roles")

			if tt.member == standardUser {
				if strings.Contains(tt.role, "project") {
					err := users.AddProjectMember(rb.client, rb.adminProject, rb.standardUser, tt.role, nil)
					require.NoError(rb.T(), err)
				} else {
					err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, tt.role, nil)
					require.NoError(rb.T(), err)
				}
			}

			relogin, err := rb.standardUserClient.ReLogin()
			require.NoError(rb.T(), err)
			rb.standardUserClient = relogin

			//Create a steve user client for a standard user to get the cluster details
			steveStdUserclient, err := rb.standardUserClient.Steve.ProxyDownstream(rb.cluster.ID)
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
				//Set up additional user client to be added to the project
				additionalUser, err := users.CreateUserWithRole(rb.client, users.UserConfig(), standardUser)
				require.NoError(rb.T(), err)
				rb.additionalUser = additionalUser
				rb.additionalUserClient, err = rb.client.AsUser(rb.additionalUser)
				require.NoError(rb.T(), err)
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
