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
	"github.com/rancher/rancher/tests/v2/validation/provisioning/rke1"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	coreV1 "k8s.io/api/core/v1"
)

type RBTestSuite struct {
	suite.Suite
	client                *rancher.Client
	standardUser          *management.User
	standardUserClient    *rancher.Client
	session               *session.Session
	cluster               *management.Cluster
	adminProject          *management.Project
	steveAdminClient      *v1.Client
	steveStdUserclient    *v1.Client
	additionalUser        *management.User
	additionalUserClient  *rancher.Client
	standardUserCOProject *management.Project
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

	//Get cluster name from the config file and append cluster details in rb
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rb.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rb.client, clusterName)
	require.NoError(rb.T(), err, "Error getting cluster ID")
	rb.cluster, err = rb.client.Management.Cluster.ByID(clusterID)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) ValidateListCluster(role string) {

	//Testcase1 Verify cluster members - Owner/member,  Project members - Owner/member are able to list clusters
	clusterList, err := rb.standardUserClient.Steve.SteveType(clusters.ProvisioningSteveResouceType).ListAll(nil)
	require.NoError(rb.T(), err)
	clusterStatus := &apiV1.ClusterStatus{}
	err = v1.ConvertToK8sType(clusterList.Data[0].Status, clusterStatus)
	require.NoError(rb.T(), err)

	if role != restrictedAdmin {
		assert.Equal(rb.T(), 1, len(clusterList.Data))
		actualClusterID := clusterStatus.ClusterName
		assert.Equal(rb.T(), rb.cluster.ID, actualClusterID)
	} else {
		adminClusterList, err := rb.client.Steve.SteveType(clusters.ProvisioningSteveResouceType).ListAll(nil)
		require.NoError(rb.T(), err)
		assert.Equal(rb.T(), (len(adminClusterList.Data) - 1), len(clusterList.Data))
	}
}

func (rb *RBTestSuite) ValidateListProjects(role string) {

	//Testcase2 Verify members of cluster are able to list the projects in a cluster
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

	//Testcase3 Validate if cluster members can create a project in the downstream cluster
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

	//Testcase4 Validate if cluster members can create namespaces in project they are not owner of
	log.Info("Testcase4 - Validating if ", role, " can create namespace in a project they are not owner of. ")
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

	//Testcase5 Validate if cluster members/project members are able to list all the namespaces in a cluster
	log.Info("Testcase5 - Validating if ", role, " can lists all namespaces in a cluster.")

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

	//Testcase6 Validate if cluster members are able to delete the namespace in the admin created project
	log.Info("Testcase6 - Validating if ", role, " cannot delete a namespace from a project they own.")

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

	//Testcase7 Validate if project members are able to add other members in cluster
	errUserRole := users.AddClusterRoleToUser(rb.standardUserClient, rb.cluster, rb.additionalUser, roleOwner)

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

	//Testcase8 Validate if project owners/members are able to add another standard user as a project members
	errUserRole := users.AddProjectMember(rb.standardUserClient, rb.adminProject, rb.additionalUser, roleProjectOwner)

	switch role {
	case roleProjectOwner, restrictedAdmin:
		require.NoError(rb.T(), errUserRole)
		additionalUserClient, err := rb.additionalUserClient.ReLogin()
		require.NoError(rb.T(), err)
		rb.additionalUserClient = additionalUserClient

		projectList, err := listProjects(rb.standardUserClient, rb.cluster.ID)
		require.NoError(rb.T(), err)
		assert.Equal(rb.T(), 1, len(projectList))
		assert.Equal(rb.T(), rb.adminProject.Name, projectList[0])
	case roleProjectMember:
		require.Error(rb.T(), errUserRole)
	}

}

func (rb *RBTestSuite) ValidateDeleteProject(role string) {

	//Testcase9 Validate if cluster members are able to delete the admin created project
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

	//Testcase10a Remove added cluster member from the cluster as an admin
	err := users.RemoveClusterRoleFromUser(rb.client, rb.standardUser)
	require.NoError(rb.T(), err)
}

func (rb *RBTestSuite) ValidateRemoveProjectRoles() {

	//Testcase10b Remove added project member from the cluster projects as an admin
	err := users.RemoveProjectMember(rb.client, rb.standardUser)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) ValidateAddStdUserAsProjectOwner() {

	//Create project as a cluster Owner
	createProjectAsCO, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.standardUserCOProject = createProjectAsCO

	//Additional testcase1 Validate if cluster owner can add a user as project owner in a project
	err = users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, roleProjectOwner)
	require.NoError(rb.T(), err)
	userGetProject, err := projects.GetProjectList(rb.additionalUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(userGetProject.Data))
	assert.Equal(rb.T(), rb.standardUserCOProject.Name, userGetProject.Data[0].Name)

	err = users.RemoveProjectMember(rb.standardUserClient, rb.additionalUser)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) ValidateAddMemberAsClusterRoles() {

	//Additional testcase2 Validate if cluster owners should be able to add another standard user as a cluster owner
	errUserRole := users.AddClusterRoleToUser(rb.standardUserClient, rb.cluster, rb.additionalUser, roleOwner)
	require.NoError(rb.T(), errUserRole)
	additionalUserClient, err := rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.additionalUserClient = additionalUserClient

	clusterList, err := rb.additionalUserClient.Steve.SteveType(clusters.ProvisioningSteveResouceType).ListAll(nil)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))

	err = users.RemoveClusterRoleFromUser(rb.standardUserClient, rb.additionalUser)
	require.NoError(rb.T(), err)

}

func (rb *RBTestSuite) ValidateAddCMAsProjectOwner() {

	//Additional test3 Cluster owner should be able to add cluster members as a project owner

	errUserRole := users.AddClusterRoleToUser(rb.standardUserClient, rb.cluster, rb.additionalUser, roleMember)
	require.NoError(rb.T(), errUserRole)
	additionalUserClient, err := rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.additionalUserClient = additionalUserClient

	clusterList, err := rb.standardUserClient.Steve.SteveType(clusters.ProvisioningSteveResouceType).ListAll(nil)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))

	//Add the cluster member to the project created by the cluster Owner
	err = users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, roleProjectOwner)
	require.NoError(rb.T(), err)
	userGetProject, err := projects.GetProjectList(rb.additionalUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), rb.standardUserCOProject.Name, userGetProject.Data[0].Name)

}

func (rb *RBTestSuite) ValidateAddPOsAsProjectOwner() {
	createProjectAsCO, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.standardUserCOProject = createProjectAsCO

	log.Info("Additional Testcase4 - Validating if Project Owner can add another Project Owner ")
	errUserRole := users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, roleProjectOwner)
	require.NoError(rb.T(), errUserRole)
	rb.additionalUserClient, err = rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)

	addNewUserAsPO, err := createUser(rb.client, standardUser)
	require.NoError(rb.T(), err)
	addNewUserAsPOClient, err := rb.client.AsUser(addNewUserAsPO)
	require.NoError(rb.T(), err)

	errUserRole2 := users.AddProjectMember(rb.additionalUserClient, rb.standardUserCOProject, addNewUserAsPO, roleProjectOwner)
	require.NoError(rb.T(), errUserRole2)

	addNewUserAsPOClient, err = addNewUserAsPOClient.ReLogin()
	require.NoError(rb.T(), err)

	userGetProject, err := projects.GetProjectList(addNewUserAsPOClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(userGetProject.Data))
	assert.Equal(rb.T(), rb.standardUserCOProject.Name, userGetProject.Data[0].Name)

	errRemoveMember := users.RemoveProjectMember(rb.standardUserClient, addNewUserAsPO)
	require.NoError(rb.T(), errRemoveMember)

	userProjectEmptyAfterRemoval, err := projects.GetProjectList(addNewUserAsPOClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 0, len(userProjectEmptyAfterRemoval.Data))
	users.RemoveProjectMember(rb.additionalUserClient, rb.additionalUser)
}

func (rb *RBTestSuite) ValidateCannotAddMPMsAsProjectOwner() {
	//Create project as a cluster Owner
	createProjectAsCO, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.standardUserCOProject = createProjectAsCO

	log.Info("Additional Testcase5 - Validating if Manage Project Member cannot add Project Owner ")
	errUserRole := users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, roleManageProjectMember)
	require.NoError(rb.T(), errUserRole)
	rb.additionalUserClient, err = rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)

	addNewUserAsPO, err := createUser(rb.client, standardUser)
	require.NoError(rb.T(), err)
	addNewUserAsPOClient, err := rb.client.AsUser(addNewUserAsPO)
	require.NoError(rb.T(), err)

	errUserRole2 := users.AddProjectMember(rb.additionalUserClient, rb.standardUserCOProject, addNewUserAsPO, roleProjectOwner)
	require.Error(rb.T(), errUserRole2)
	errStatus := strings.Split(errUserRole2.Error(), ".")[1]
	rgx := regexp.MustCompile(`\[(.*?)\]`)
	errorMsg := rgx.FindStringSubmatch(errStatus)
	assert.Equal(rb.T(), "422 Unprocessable Entity", errorMsg[1])

	addNewUserAsPOClient, err = addNewUserAsPOClient.ReLogin()
	require.NoError(rb.T(), err)

	userGetProject, err := projects.GetProjectList(addNewUserAsPOClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 0, len(userGetProject.Data))
}

func (rb *RBTestSuite) ValidateListGlobalSettings() {
	adminListSettings, err := listGlobalSettings(rb.steveAdminClient)
	require.NoError(rb.T(), err)
	resAdminListSettings, err := listGlobalSettings(rb.steveStdUserclient)
	require.NoError(rb.T(), err)

	assert.Equal(rb.T(), len(adminListSettings), len(resAdminListSettings))
	assert.Equal(rb.T(), adminListSettings, resAdminListSettings)
}

func (rb *RBTestSuite) ValidateEditGlobalSettings() {
	kubeConfigTokenSetting, err := rb.steveStdUserclient.SteveType("management.cattle.io.setting").ByID(kubeConfigTokenSettingID)
	require.NoError(rb.T(), err)

	_, err = editGlobalSettings(rb.steveStdUserclient, kubeConfigTokenSetting, "3")
	require.Error(rb.T(), err)
	errMessage := strings.Split(err.Error(), ":")[0]
	assert.Equal(rb.T(), "Resource type [management.cattle.io.setting] is not updatable", errMessage)

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
			newUser, err := createUser(rb.client, tt.member)
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
		rb.Run("Test case Validate if users can list any downstream clusters before adding the cluster role "+tt.name, func() {
			_, err := rb.standardUserClient.Steve.SteveType(clusters.ProvisioningSteveResouceType).ListAll(nil)
			if tt.member == standardUser {
				require.Error(rb.T(), err)
				assert.Contains(rb.T(), "Resource type [provisioning.cattle.io.cluster] has no method GET", err.Error())
			}
		})

		rb.Run("Adding user as "+tt.name+" to the downstream cluster.", func() {
			//Adding created user to the downstream clusters with the specified roles.

			if tt.member == standardUser {
				if strings.Contains(tt.role, "project") {
					err := users.AddProjectMember(rb.client, rb.adminProject, rb.standardUser, tt.role)
					require.NoError(rb.T(), err)
				} else {
					err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, tt.role)
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

		rb.Run("Testcase1 - Validating the cluster count obtained as the role "+tt.name, func() {
			rb.ValidateListCluster(tt.role)
		})

		rb.Run("Testcase2 - Validating if members with role "+tt.name+" are able to list all projects", func() {
			rb.ValidateListProjects(tt.role)
		})

		rb.Run("Testcase3 - Validating if members with role "+tt.name+" is able to create a project in the cluster", func() {
			rb.ValidateCreateProjects(tt.role)

		})

		rb.Run("Testcase 4 through 6 - Validate namespaces checks for members with role "+tt.name, func() {
			rb.ValidateNS(tt.role)
		})

		if !strings.Contains(tt.role, "cluster") {
			rb.Run("Testcase7 - Validating if member with role "+tt.name+" can add members to the cluster", func() {
				//Set up additional user client to be added to the project
				additionalUser, err := createUser(rb.client, tt.member)
				require.NoError(rb.T(), err)
				rb.additionalUser = additionalUser
				rb.additionalUserClient, err = rb.client.AsUser(rb.additionalUser)
				require.NoError(rb.T(), err)
				rb.ValidateAddClusterRoles(tt.role)
			})

			rb.Run("Testcase8 - Validating if member with role "+tt.name+" can add members to the project", func() {
				rb.ValidateAddProjectRoles(tt.role)
			})
		}

		rb.Run("Testcase9 - Validating if member with role "+tt.name+" can delete a project they are not owner of ", func() {
			rb.ValidateDeleteProject(tt.role)
		})

		rb.Run("Testcase10 - Validating if member with role "+tt.name+" is removed from the cluster and returns nil clusters", func() {
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

func (rb *RBTestSuite) TestRBACAdditional() {

	tests := []struct {
		name   string
		member string
	}{
		{"Standard User", standardUser},
		{"Restricted Admin", restrictedAdmin},
	}

	for _, tt := range tests {
		rb.Run("Set up User with cluster Role for additional rbac test cases "+roleOwner, func() {
			newUser, err := createUser(rb.client, tt.member)
			require.NoError(rb.T(), err)
			rb.standardUser = newUser
			rb.T().Logf("Created user: %v", rb.standardUser.Username)
			rb.standardUserClient, err = rb.client.AsUser(newUser)
			require.NoError(rb.T(), err)
		})

		if tt.member == standardUser {
			rb.T().Logf("Adding user as " + roleOwner + " to the downstream cluster.")
			//Adding created user to the downstream clusters with the role cluster Owner.
			err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, roleOwner)
			require.NoError(rb.T(), err)
			rb.standardUserClient, err = rb.standardUserClient.ReLogin()
			require.NoError(rb.T(), err)

			//Setting up an additional user for the additional rbac cases
			additionalUser, err := createUser(rb.client, standardUser)
			require.NoError(rb.T(), err)
			rb.additionalUser = additionalUser
			rb.additionalUserClient, err = rb.client.AsUser(rb.additionalUser)
			require.NoError(rb.T(), err)

			rb.Run("Additional testcase1 - Validating if member with role "+roleOwner+" can add another standard user as a project owner", func() {
				rb.ValidateAddStdUserAsProjectOwner()
			})

			rb.Run("Additional testcase2 - Validating if member with role "+roleOwner+" can add another standard user as a cluster owner", func() {
				rb.ValidateAddMemberAsClusterRoles()
			})

			rb.Run("Additional testcase3 - Validating if member with role "+roleOwner+" can add a cluster member as a project owner", func() {
				rb.ValidateAddCMAsProjectOwner()
			})

			rb.Run("Additional testcase4 - Validating if member with role "+roleProjectOwner+" can add a project owner", func() {
				rb.ValidateAddPOsAsProjectOwner()
			})

			rb.Run("Additional testcase5 - Validating if member with role "+roleManageProjectMember+" can not add a project owner", func() {
				rb.ValidateCannotAddMPMsAsProjectOwner()
			})

		} else {
			rb.Run("Additional testcase6 - Validating if "+restrictedAdmin+" can create an RKE1 cluster", func() {
				clusterConfig := getClusterConfig()
				rke1.TestProvisioningRKE1CustomCluster(rb.T(), rb.standardUserClient, clusterConfig.externalNodeProvider,
					clusterConfig.nodeRoles, "", clusterConfig.kubernetesVersion, clusterConfig.cni, clusterConfig.advancedOptions)
			})

			rb.Run("Additional testcase7 - Validating if "+restrictedAdmin+" can list global settings", func() {
				//Steve client is required to list global settings.
				rb.steveStdUserclient = rb.standardUserClient.Steve
				rb.steveAdminClient = rb.client.Steve

				rb.ValidateListGlobalSettings()
			})

			rb.Run("Additional testcase8 - Validating if "+restrictedAdmin+" can edit global settings", func() {
				rb.ValidateEditGlobalSettings()
			})
		}
	}
}

func TestRBACTestSuite(t *testing.T) {
	suite.Run(t, new(RBTestSuite))
}
