//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package rbac

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/kubeapi/rbac"
	"github.com/rancher/shepherd/extensions/projects"
	"github.com/rancher/shepherd/extensions/provisioning"
	"github.com/rancher/shepherd/extensions/provisioninginput"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RBACAdditionalTestSuite struct {
	suite.Suite
	client                *rancher.Client
	standardUser          *management.User
	standardUserClient    *rancher.Client
	session               *session.Session
	cluster               *management.Cluster
	steveAdminClient      *v1.Client
	steveStdUserclient    *v1.Client
	additionalUser        *management.User
	additionalUserClient  *rancher.Client
	standardUserCOProject *management.Project
}

func (rb *RBACAdditionalTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *RBACAdditionalTestSuite) SetupSuite() {
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

func (rb *RBACAdditionalTestSuite) ValidateAddStdUserAsProjectOwner() {

	createProjectAsCO, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.standardUserCOProject = createProjectAsCO

	log.Info("Validating if cluster owner can add a user as project owner in a project")
	err = users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, roleProjectOwner, nil)
	require.NoError(rb.T(), err)
	userGetProject, err := projects.GetProjectList(rb.additionalUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(userGetProject.Data))
	assert.Equal(rb.T(), rb.standardUserCOProject.Name, userGetProject.Data[0].Name)

	err = users.RemoveProjectMember(rb.standardUserClient, rb.additionalUser)
	require.NoError(rb.T(), err)
}

func (rb *RBACAdditionalTestSuite) ValidateAddMemberAsClusterRoles() {

	log.Info("Validating if cluster owners should be able to add another standard user as a cluster owner")
	errUserRole := users.AddClusterRoleToUser(rb.standardUserClient, rb.cluster, rb.additionalUser, roleOwner, nil)
	require.NoError(rb.T(), errUserRole)
	additionalUserClient, err := rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.additionalUserClient = additionalUserClient

	clusterList, err := rb.additionalUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))

	err = users.RemoveClusterRoleFromUser(rb.standardUserClient, rb.additionalUser)
	require.NoError(rb.T(), err)

}

func (rb *RBACAdditionalTestSuite) ValidateAddCMAsProjectOwner() {

	log.Info("Validating if cluster manage member should be able to add as a project member")
	errUserRole := users.AddClusterRoleToUser(rb.standardUserClient, rb.cluster, rb.additionalUser, roleMember, nil)
	require.NoError(rb.T(), errUserRole)
	additionalUserClient, err := rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.additionalUserClient = additionalUserClient

	clusterList, err := rb.standardUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))

	err = users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, roleProjectOwner, nil)
	require.NoError(rb.T(), err)
	userGetProject, err := projects.GetProjectList(rb.additionalUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), rb.standardUserCOProject.Name, userGetProject.Data[0].Name)

}

func (rb *RBACAdditionalTestSuite) ValidateAddPOsAsProjectOwner() {
	createProjectAsCO, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.standardUserCOProject = createProjectAsCO

	log.Info("Validating if Project Owner can add another Project Owner")
	errUserRole := users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, roleProjectOwner, nil)
	require.NoError(rb.T(), errUserRole)
	rb.additionalUserClient, err = rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)

	addNewUserAsPO, err := users.CreateUserWithRole(rb.client, users.UserConfig(), standardUser)
	require.NoError(rb.T(), err)
	addNewUserAsPOClient, err := rb.client.AsUser(addNewUserAsPO)
	require.NoError(rb.T(), err)

	errUserRole2 := users.AddProjectMember(rb.additionalUserClient, rb.standardUserCOProject, addNewUserAsPO, roleProjectOwner, nil)
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

func (rb *RBACAdditionalTestSuite) ValidateCannotAddMPMsAsProjectOwner() {
	createProjectAsCO, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.standardUserCOProject = createProjectAsCO

	log.Info("Validating if Manage Project Member cannot add Project Owner")
	errUserRole := users.AddProjectMember(rb.standardUserClient, rb.standardUserCOProject, rb.additionalUser, roleCustomManageProjectMember, nil)
	require.NoError(rb.T(), errUserRole)
	rb.additionalUserClient, err = rb.additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)

	addNewUserAsPO, err := users.CreateUserWithRole(rb.client, users.UserConfig(), standardUser)
	require.NoError(rb.T(), err)
	addNewUserAsPOClient, err := rb.client.AsUser(addNewUserAsPO)
	require.NoError(rb.T(), err)

	errUserRole2 := users.AddProjectMember(rb.additionalUserClient, rb.standardUserCOProject, addNewUserAsPO, roleProjectOwner, nil)
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

func (rb *RBACAdditionalTestSuite) ValidateListGlobalSettings() {
	adminListSettings, err := listGlobalSettings(rb.steveAdminClient)
	require.NoError(rb.T(), err)
	resAdminListSettings, err := listGlobalSettings(rb.steveStdUserclient)
	require.NoError(rb.T(), err)

	assert.Equal(rb.T(), len(adminListSettings), len(resAdminListSettings))
	assert.Equal(rb.T(), adminListSettings, resAdminListSettings)
}

func (rb *RBACAdditionalTestSuite) ValidateEditGlobalSettings() {
	kubeConfigTokenSetting, err := rb.steveStdUserclient.SteveType("management.cattle.io.setting").ByID(kubeConfigTokenSettingID)
	require.NoError(rb.T(), err)

	_, err = editGlobalSettings(rb.steveStdUserclient, kubeConfigTokenSetting, "3")
	require.Error(rb.T(), err)
	errMessage := strings.Split(err.Error(), ":")[0]
	assert.Equal(rb.T(), "Resource type [management.cattle.io.setting] is not updatable", errMessage)

}

func (rb *RBACAdditionalTestSuite) ValidateListGlobalRoles() {

	expectedError := "globalroles.management.cattle.io is forbidden: User \"" + rb.standardUser.ID + "\" cannot list resource \"globalroles\" in API group \"management.cattle.io\" at the cluster scope"
	_, err := rbac.ListGlobalRoles(rb.standardUserClient, metav1.ListOptions{})
	require.Error(rb.T(), err)
	assert.Equal(rb.T(), expectedError, err.Error())
}

func (rb *RBACAdditionalTestSuite) TestRBACAdditional() {

	tests := []struct {
		name   string
		member string
	}{
		{"Standard User RBAC Additional", standardUser},
		{"Restricted Admin RBAC Additional", restrictedAdmin},
	}

	for _, tt := range tests {
		rb.Run("Set up User with cluster Role for additional rbac test cases "+roleOwner, func() {
			newUser, err := users.CreateUserWithRole(rb.client, users.UserConfig(), tt.member)
			require.NoError(rb.T(), err)
			rb.standardUser = newUser
			rb.T().Logf("Created user: %v", rb.standardUser.Username)
			rb.standardUserClient, err = rb.client.AsUser(newUser)
			require.NoError(rb.T(), err)
		})

		if tt.member == standardUser {
			rb.T().Logf("Adding user as " + roleOwner + " to the downstream cluster.")
			//Adding created user to the downstream clusters with the role cluster Owner.
			err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, roleOwner, nil)
			require.NoError(rb.T(), err)
			rb.standardUserClient, err = rb.standardUserClient.ReLogin()
			require.NoError(rb.T(), err)

			//Setting up an additional user for the additional rbac cases
			additionalUser, err := users.CreateUserWithRole(rb.client, users.UserConfig(), standardUser)
			require.NoError(rb.T(), err)
			rb.additionalUser = additionalUser
			rb.additionalUserClient, err = rb.client.AsUser(rb.additionalUser)
			require.NoError(rb.T(), err)

			rb.Run("Validating if member with role "+roleOwner+" can add another standard user as a project owner", func() {
				rb.ValidateAddStdUserAsProjectOwner()
			})

			rb.Run("Validating if member with role "+roleOwner+" can add another standard user as a cluster owner", func() {
				rb.ValidateAddMemberAsClusterRoles()
			})

			rb.Run("Validating if member with role "+roleOwner+" can add a cluster member as a project owner", func() {
				rb.ValidateAddCMAsProjectOwner()
			})

			rb.Run("Validating if member with role "+roleProjectOwner+" can add a project owner", func() {
				rb.ValidateAddPOsAsProjectOwner()
			})

			rb.Run("Validating if member with role "+roleCustomManageProjectMember+" can not add a project owner", func() {
				rb.ValidateCannotAddMPMsAsProjectOwner()
			})

			rb.Run("Validating if standard users can get global roles", func() {
				rb.ValidateListGlobalRoles()
			})

		} else {
			rb.Run("Validating if "+restrictedAdmin+" can create an RKE1 cluster", func() {
				userConfig := new(provisioninginput.Config)
				config.LoadConfig(provisioninginput.ConfigurationFileKey, userConfig)
				nodeProviders := userConfig.NodeProviders[0]
				nodeAndRoles := []provisioninginput.NodePools{
					provisioninginput.AllRolesNodePool,
				}
				externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviders)
				clusterConfig := clusters.ConvertConfigToClusterConfig(userConfig)
				clusterConfig.NodePools = nodeAndRoles
				kubernetesVersion, err := kubernetesversions.Default(rb.client, clusters.RKE1ClusterType.String(), []string{})
				require.NoError(rb.T(), err)

				clusterConfig.KubernetesVersion = kubernetesVersion[0]
				clusterConfig.CNI = userConfig.CNIs[0]
				clusterObject, _, err := provisioning.CreateProvisioningRKE1CustomCluster(rb.client, &externalNodeProvider, clusterConfig)
				require.NoError(rb.T(), err)
				provisioning.VerifyRKE1Cluster(rb.T(), rb.client, clusterConfig, clusterObject)
			})

			rb.Run("Validating if "+restrictedAdmin+" can list global settings", func() {
				//Steve client is required to list global settings.
				rb.steveStdUserclient = rb.standardUserClient.Steve
				rb.steveAdminClient = rb.client.Steve

				rb.ValidateListGlobalSettings()
			})

			rb.Run("Validating if "+restrictedAdmin+" can edit global settings", func() {
				rb.ValidateEditGlobalSettings()
			})
		}
	}
}

func TestRBACAdditionalTestSuite(t *testing.T) {
	suite.Run(t, new(RBACAdditionalTestSuite))
}
