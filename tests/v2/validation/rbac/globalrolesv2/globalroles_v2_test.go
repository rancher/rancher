//go:build (validation || infra.any || cluster.any || stress) && !sanity && !extended

package globalrolesv2

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	rbacapi "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/users"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

type GlobalRolesV2TestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (gr *GlobalRolesV2TestSuite) TearDownSuite() {
	gr.session.Cleanup()
}

func (gr *GlobalRolesV2TestSuite) SetupSuite() {
	testSession := session.NewSession()
	gr.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(gr.T(), err)

	gr.client = client
}

func (gr *GlobalRolesV2TestSuite) validateRBACResources(createdUser *management.User, inheritedRoles []string) (string, int) {
	log.Info("Verify that the global role binding is created for the user.")
	grbOwner, err := getGlobalRoleBindingForUserWrangler(gr.client, createdUser.ID)
	require.NoError(gr.T(), err)
	require.NotEmpty(gr.T(), grbOwner, "Global Role Binding not found for the user")
	grbName := grbOwner

	log.Info("Verify that the cluster role template bindings are created for the downstream clusters.")
	clusterNames, err := clusters.ListDownstreamClusters(gr.client)
	require.NoError(gr.T(), err)
	clusterCount := len(clusterNames)
	expectedCrtbCount := clusterCount * len(inheritedRoles)
	crtbs, err := listClusterRoleTemplateBindingsForInheritedClusterRoles(gr.client, grbOwner, expectedCrtbCount)
	require.NoError(gr.T(), err)
	actualCrtbCount := len(crtbs.Items)
	require.Equal(gr.T(), expectedCrtbCount, actualCrtbCount, "Unexpected number of ClusterRoleTemplateBindings: Expected %d, Actual %d", expectedCrtbCount, actualCrtbCount)

	log.Info("Verify that the cluster role bindings are created for the downstream cluster.")
	expectedCrbCount := expectedCrtbCount
	crbs, err := getCRBsForCRTBs(gr.client, crtbs)
	require.NoError(gr.T(), err)
	actualCrbCount := len(crbs.Items)
	require.Equal(gr.T(), expectedCrbCount, actualCrbCount, "Unexpected number of ClusterRoleBindings: Expected %d, Actual %d", expectedCrbCount, actualCrbCount)

	log.Info("Verify that the role bindings are created for the downstream cluster.")
	expectedRbCount := expectedCrtbCount
	rbs, err := getRBsForCRTBs(gr.client, crtbs)
	require.NoError(gr.T(), err)
	actualRbCount := len(rbs.Items)
	require.Equal(gr.T(), expectedRbCount, actualRbCount, "Unexpected number of RoleBindings: Expected %d, Actual %d", expectedRbCount, actualRbCount)
	return grbName, clusterCount
}

func (gr *GlobalRolesV2TestSuite) TestCreateUserWithInheritedClusterRoles() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{roleOwner}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)

	gr.validateRBACResources(createdUser, inheritedClusterRoles)
}

func (gr *GlobalRolesV2TestSuite) TestCreateUserWithMultipleInheritedClusterRoles() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{roleCrtbView, roleProjectsCreate, roleProjectsView}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)

	gr.validateRBACResources(createdUser, inheritedClusterRoles)
}

func (gr *GlobalRolesV2TestSuite) TestCreateUserWithInheritedCustomClusterRole() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a custom role that inherits rules from the cluster-owner.")
	inheritedRoleTemplateName := namegen.AppendRandomString("crole")
	inheritedRoleTemplate, err := gr.client.Management.RoleTemplate.Create(&management.RoleTemplate{
		Context:         "cluster",
		Name:            inheritedRoleTemplateName,
		RoleTemplateIDs: []string{roleOwner},
	})
	require.NoError(gr.T(), err)

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{inheritedRoleTemplate.ID}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)

	_, expectedClusterCount := gr.validateRBACResources(createdUser, inheritedClusterRoles)

	log.Info("Verify that the user can list all the downstream clusters.")
	userClient, err := gr.client.AsUser(createdUser)
	require.NoError(gr.T(), err)
	clusterNames, err := clusters.ListDownstreamClusters(userClient)
	require.NoError(gr.T(), err)
	actualClusterCount := len(clusterNames)
	require.Equal(gr.T(), expectedClusterCount, actualClusterCount, "Unexpected number of Clusters: Expected %d, Actual %d", expectedClusterCount, actualClusterCount)
}

func (gr *GlobalRolesV2TestSuite) TestClusterCreationAfterAddingGlobalRoleWithInheritedClusterRoles() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{roleMember}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)

	_, expectedClusterCount := gr.validateRBACResources(createdUser, inheritedClusterRoles)

	log.Info("Verify that the user can list all the downstream clusters.")
	userClient, err := gr.client.AsUser(createdUser)
	require.NoError(gr.T(), err)
	clusterNames, err := clusters.ListDownstreamClusters(userClient)
	require.NoError(gr.T(), err)
	actualClusterCount := len(clusterNames)
	require.Equal(gr.T(), expectedClusterCount, actualClusterCount, "Unexpected number of Clusters: Expected %d, Actual %d", expectedClusterCount, actualClusterCount)

	log.Info("As the new user, create new downstream clusters.")
	clusterObject, _, testClusterConfig, err := createDownstreamCluster(userClient, "RKE1")
	require.NoError(gr.T(), err)
	provisioning.VerifyRKE1Cluster(gr.T(), userClient, testClusterConfig, clusterObject)
	_, steveObject, testClusterConfig, err := createDownstreamCluster(userClient, "RKE2")
	require.NoError(gr.T(), err)
	provisioning.VerifyCluster(gr.T(), userClient, testClusterConfig, steveObject)

	gr.validateRBACResources(createdUser, inheritedClusterRoles)
}

func (gr *GlobalRolesV2TestSuite) TestUpdateExistingUserWithCustomGlobalRoleInheritingClusterRoles() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{roleOwner}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser)
	require.NoError(gr.T(), err)

	log.Info("Add the new global role with inheritedClusterRoles to the user.")
	globalRoleBinding.Name = namegen.AppendRandomString("testgrb")
	globalRoleBinding.UserName = createdUser.ID
	globalRoleBinding.GlobalRoleName = createdGlobalRole.Name
	_, err = rbacapi.CreateGlobalRoleBinding(gr.client, globalRoleBinding)
	require.NoError(gr.T(), err)

	_, expectedClusterCount := gr.validateRBACResources(createdUser, inheritedClusterRoles)

	log.Info("Verify that the user can list all the downstream clusters.")
	userClient, err := gr.client.AsUser(createdUser)
	require.NoError(gr.T(), err)
	clusterNames, err := clusters.ListDownstreamClusters(userClient)
	require.NoError(gr.T(), err)
	actualClusterCount := len(clusterNames)
	require.Equal(gr.T(), expectedClusterCount, actualClusterCount, "Unexpected number of Clusters: Expected %d, Actual %d", expectedClusterCount, actualClusterCount)
}

func (gr *GlobalRolesV2TestSuite) TestUserDeletionAndResourceCleanupWithInheritedClusterRoles() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{roleOwner}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)

	grbName, _ := gr.validateRBACResources(createdUser, inheritedClusterRoles)

	log.Infof("Delete the user: %s.", createdUser.Username)
	err = gr.client.Management.User.Delete(createdUser)
	require.NoError(gr.T(), err)

	log.Infof("Verify that the global role %s is not deleted.", createdGlobalRole.Name)
	listOpt := metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdGlobalRole.Name,
	}
	grList, err := rbacapi.ListGlobalRoles(gr.client, listOpt)
	require.NoError(gr.T(), err)
	require.NotEmpty(gr.T(), grList, "Global Role does not exist.")

	log.Infof("Verify that the global role binding %s is deleted for the user.", grbName)
	var grbOwner string
	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		grbOwner, pollErr = getGlobalRoleBindingForUserWrangler(gr.client, createdUser.ID)
		if pollErr != nil {
			return false, pollErr
		}
		if grbOwner == "" {
			return true, nil
		}
		return false, nil
	})
	require.NoError(gr.T(), err)
	require.Empty(gr.T(), grbOwner, "Global Role Binding exists for the user.")

	log.Info("Verify that the cluster role template bindings are deleted for the downstream clusters.")
	expectedCrtbCount := 0
	crtbs, err := listClusterRoleTemplateBindingsForInheritedClusterRoles(gr.client, grbOwner, expectedCrtbCount)
	require.NoError(gr.T(), err)
	actualCrtbCount := len(crtbs.Items)
	require.Equal(gr.T(), expectedCrtbCount, actualCrtbCount, "Unexpected number of ClusterRoleTemplateBindings: Expected %d, Actual %d", expectedCrtbCount, actualCrtbCount)

	log.Info("Verify that the cluster role bindings are deleted for the downstream cluster.")
	crbs, err := getCRBsForCRTBs(gr.client, crtbs)
	require.NoError(gr.T(), err)
	actualCrbCount := len(crbs.Items)
	require.Equal(gr.T(), 0, actualCrbCount, "Unexpected number of ClusterRoleBindings: Expected %d, Actual %d", 0, actualCrbCount)

	log.Info("Verify that the role bindings are deleted for the downstream cluster.")
	rbs, err := getRBsForCRTBs(gr.client, crtbs)
	require.NoError(gr.T(), err)
	actualRbCount := len(rbs.Items)
	require.Equal(gr.T(), 0, actualRbCount, "Unexpected number of RoleBindings: Expected %d, Actual %d", 0, actualRbCount)
}

func (gr *GlobalRolesV2TestSuite) TestUserWithInheritedClusterRolesImpactFromDeletingGlobalRoleBinding() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{roleOwner}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)

	grbName, expectedClusterCount := gr.validateRBACResources(createdUser, inheritedClusterRoles)

	log.Info("Verify that the user can list all the downstream clusters.")
	userClient, err := gr.client.AsUser(createdUser)
	require.NoError(gr.T(), err)
	clusterNames, err := clusters.ListDownstreamClusters(userClient)
	require.NoError(gr.T(), err)
	actualClusterCount := len(clusterNames)
	require.Equal(gr.T(), expectedClusterCount, actualClusterCount, "Unexpected number of Clusters: Expected %d, Actual %d", expectedClusterCount, actualClusterCount)

	log.Info("Delete Global Role Binding.")
	err = rbacapi.DeleteGlobalRoleBinding(gr.client, grbName)
	require.NoError(gr.T(), err)

	log.Info("Verify that the global role is not deleted.")
	listOpt := metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdGlobalRole.Name,
	}
	grList, err := rbacapi.ListGlobalRoles(gr.client, listOpt)
	require.NoError(gr.T(), err)
	require.NotEmpty(gr.T(), grList, "Global Role does not exist.")

	log.Info("Verify that the global role binding is deleted for the user.")
	grbOwner, err := getGlobalRoleBindingForUserWrangler(gr.client, createdUser.ID)
	require.NoError(gr.T(), err)
	require.Empty(gr.T(), grbOwner, "Global Role Binding exists for the user.")

	log.Info("Verify that the cluster role template bindings are deleted for the downstream clusters.")
	expectedCrtbCount := 0
	crtbs, err := listClusterRoleTemplateBindingsForInheritedClusterRoles(gr.client, grbOwner, expectedCrtbCount)
	require.NoError(gr.T(), err)
	actualCrtbCount := len(crtbs.Items)
	require.Equal(gr.T(), expectedCrtbCount, actualCrtbCount, "Unexpected number of ClusterRoleTemplateBindings: Expected %d, Actual %d", expectedCrtbCount, actualCrtbCount)

	log.Info("Verify that the cluster role bindings are deleted for the downstream cluster.")
	crbs, err := getCRBsForCRTBs(gr.client, crtbs)
	require.NoError(gr.T(), err)
	actualCrbCount := len(crbs.Items)
	require.Equal(gr.T(), 0, actualCrbCount, "Unexpected number of ClusterRoleBindings: Expected %d, Actual %d", 0, actualCrbCount)

	log.Info("Verify that the role bindings are deleted for the downstream cluster.")
	rbs, err := getRBsForCRTBs(gr.client, crtbs)
	require.NoError(gr.T(), err)
	actualRbCount := len(rbs.Items)
	require.Equal(gr.T(), 0, actualRbCount, "Unexpected number of RoleBindings: Expected %d, Actual %d", 0, actualRbCount)

	log.Infof("Verify that user %s cannot list the downstream clusters.", createdUser.ID)
	clusterNames, err = clusters.ListDownstreamClusters(userClient)
	require.NoError(gr.T(), err)
	actualClusterCount = len(clusterNames)
	require.Equal(gr.T(), 0, actualClusterCount, "Unexpected number of Clusters: Expected %d, Actual %d", 0, actualClusterCount)
}

func (gr *GlobalRolesV2TestSuite) TestUserWithInheritedClusterRolesImpactFromDeletingInheritedClusterRoles() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{roleMember}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)

	_, expectedClusterCount := gr.validateRBACResources(createdUser, inheritedClusterRoles)

	log.Info("Create another user with global role standard user and the same global role as the first user.")
	secondUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)
	gr.validateRBACResources(secondUser, inheritedClusterRoles)
	users := []*management.User{createdUser, secondUser}

	for _, user := range users {
		log.Infof("Verify that user %s can list all the downstream clusters.", user.ID)
		userClient, err := gr.client.AsUser(user)
		require.NoError(gr.T(), err)
		clusterNames, err := clusters.ListDownstreamClusters(userClient)
		require.NoError(gr.T(), err)
		actualClusterCount := len(clusterNames)
		require.Equal(gr.T(), expectedClusterCount, actualClusterCount, "Unexpected number of Clusters for user %s. Expected %d, Actual %d.", user.ID, expectedClusterCount, actualClusterCount)
	}

	log.Info("Remove InheritedClusterRoles from the global role.")
	updateGlobalRole := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: createdGlobalRole.Name,
		},
		InheritedClusterRoles: []string{},
	}
	_, err = rbacapi.UpdateGlobalRole(gr.client, &updateGlobalRole)
	require.NoError(gr.T(), err)

	for _, user := range users {
		log.Infof("Verify that the global role binding is not deleted for user %s.", user.ID)
		grbOwner, err := getGlobalRoleBindingForUserWrangler(gr.client, user.ID)
		require.NoError(gr.T(), err)
		require.NotEmpty(gr.T(), grbOwner, "Global Role Binding does not exist for user %s", user.ID)

		log.Infof("Verify that the cluster role template bindings are deleted for user %s.", user.ID)
		crtbs, err := listClusterRoleTemplateBindingsForInheritedClusterRoles(gr.client, grbOwner, 0)
		require.NoError(gr.T(), err)
		actualCrtbCount := len(crtbs.Items)
		require.Equal(gr.T(), 0, actualCrtbCount, "Unexpected number of ClusterRoleTemplateBindings for user %s: Expected %d, Actual %d", user.ID, 0, actualCrtbCount)

		log.Infof("Verify that the cluster role bindings are deleted for the downstream cluster.")
		crbs, err := getCRBsForCRTBs(gr.client, crtbs)
		require.NoError(gr.T(), err)
		actualCrbCount := len(crbs.Items)
		require.Equal(gr.T(), 0, actualCrbCount, "Unexpected number of ClusterRoleBindings: Expected %d, Actual %d", 0, actualCrbCount)

		log.Info("Verify that the role bindings are deleted for the downstream cluster.")
		rbs, err := getRBsForCRTBs(gr.client, crtbs)
		require.NoError(gr.T(), err)
		actualRbCount := len(rbs.Items)
		require.Equal(gr.T(), 0, actualRbCount, "Unexpected number of RoleBindings: Expected %d, Actual %d", 0, actualRbCount)

		log.Infof("Verify that user %s cannot list the downstream clusters.", user.ID)
		userClient, err := gr.client.AsUser(user)
		require.NoError(gr.T(), err)
		clusterNames, err := clusters.ListDownstreamClusters(userClient)
		require.Error(gr.T(), err)
		actualClusterCount := len(clusterNames)
		require.Equal(gr.T(), 0, actualClusterCount, "Unexpected number of Clusters: Expected %d, Actual %d", 0, actualClusterCount)
	}
}

func (gr *GlobalRolesV2TestSuite) TestUserWithInheritedClusterRolesImpactFromClusterDeletion() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a RKE2 downstream cluster.")
	_, rke2SteveObject, testClusterConfig, err := createDownstreamCluster(gr.client, "RKE2")
	require.NoError(gr.T(), err)
	provisioning.VerifyCluster(gr.T(), gr.client, testClusterConfig, rke2SteveObject)

	log.Info("Create a global role with inheritedClusterRoles.")
	inheritedClusterRoles := []string{roleOwner}
	createdGlobalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(gr.client, inheritedClusterRoles)
	require.NoError(gr.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdGlobalRole.Name)
	require.NoError(gr.T(), err)

	_, expectedClusterCount := gr.validateRBACResources(createdUser, inheritedClusterRoles)

	log.Info("Verify that the user can list all the downstream clusters.")
	userClient, err := gr.client.AsUser(createdUser)
	require.NoError(gr.T(), err)
	clusterNames, err := clusters.ListDownstreamClusters(userClient)
	require.NoError(gr.T(), err)
	actualClusterCount := len(clusterNames)
	require.Equal(gr.T(), expectedClusterCount, actualClusterCount, "Unexpected number of Clusters: Expected %d, Actual %d", expectedClusterCount, actualClusterCount)

	log.Info("Delete the RKE2 downstream cluster.")
	err = clusters.DeleteK3SRKE2Cluster(userClient, rke2SteveObject.ID)
	require.NoError(gr.T(), err)

	log.Info("Verify that the global role is not deleted.")
	listOpt := metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdGlobalRole.Name,
	}
	grList, err := rbacapi.ListGlobalRoles(gr.client, listOpt)
	require.NoError(gr.T(), err)
	require.NotEmpty(gr.T(), grList, "Global Role does not exist.")

	log.Info("Verify that the global role binding is not deleted for the user.")
	grbOwner, err := getGlobalRoleBindingForUserWrangler(gr.client, createdUser.ID)
	require.NoError(gr.T(), err)
	require.NotEmpty(gr.T(), grbOwner, "Global Role Binding does not exist for the user.")

	log.Info("Verify that the cluster role template bindings are deleted for the downstream cluster.")
	expectedCrtbCount := actualClusterCount - 1
	crtbs, err := listClusterRoleTemplateBindingsForInheritedClusterRoles(gr.client, grbOwner, expectedCrtbCount)
	require.NoError(gr.T(), err)
	actualCrtbCount := len(crtbs.Items)
	require.Equal(gr.T(), expectedCrtbCount, actualCrtbCount, "Unexpected number of ClusterRoleTemplateBindings: Expected %d, Actual %d", expectedCrtbCount, actualCrtbCount)

	log.Info("Verify that the cluster role bindings are deleted for the downstream cluster.")
	expectedCrbCount := expectedCrtbCount
	crbs, err := getCRBsForCRTBs(gr.client, crtbs)
	require.NoError(gr.T(), err)
	actualCrbCount := len(crbs.Items)
	require.Equal(gr.T(), expectedCrbCount, actualCrbCount, "Unexpected number of ClusterRoleBindings: Expected %d, Actual %d", expectedCrbCount, actualCrbCount)

	log.Info("Verify that the role bindings are deleted for the downstream cluster.")
	expectedRbCount := expectedCrtbCount
	rbs, err := getRBsForCRTBs(gr.client, crtbs)
	require.NoError(gr.T(), err)
	actualRbCount := len(rbs.Items)
	require.Equal(gr.T(), expectedRbCount, actualRbCount, "Unexpected number of RoleBindings: Expected %d, Actual %d", expectedRbCount, actualRbCount)
}

func TestGlobalRolesV2TestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRolesV2TestSuite))
}
