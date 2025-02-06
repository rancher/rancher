//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package globalroles

import (
	"fmt"
	"testing"

	rbacapi "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GlobalRolesTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (gr *GlobalRolesTestSuite) TearDownSuite() {
	gr.session.Cleanup()
}

func (gr *GlobalRolesTestSuite) SetupSuite() {
	gr.session = session.NewSession()

	client, err := rancher.NewClient("", gr.session)
	require.NoError(gr.T(), err)
	gr.client = client

	log.Info("Getting cluster name from the config file and append cluster details in gr")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(gr.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(gr.client, clusterName)
	require.NoError(gr.T(), err, "Error getting cluster ID")
	gr.cluster, err = gr.client.Management.Cluster.ByID(clusterID)
	require.NoError(gr.T(), err)
}

func (gr *GlobalRolesTestSuite) TestGlobalRoleCustom() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a user with custom global role.")
	createdGlobalRole, createdUser, err := createCustomGlobalRoleAndUser(gr.client)
	require.NoError(gr.T(), err, "Failed to create custom global role and user")

	log.Info("Verify that the global role binding is created for the user.")
	grb, err := rbac.GetGlobalRoleBindingByUserAndRole(gr.client, createdUser.ID, createdGlobalRole.Name)
	require.NoError(gr.T(), err, "Failed to get global role binding")
	require.NotEmpty(gr.T(), grb, "Global Role Binding not found for the user")

	log.Info("As admin, create a deployment.")
	_, namespace, err := projects.CreateProjectAndNamespace(gr.client, rbac.LocalCluster)
	require.NoError(gr.T(), err, "Failed to create project and namespace")
	createdDeployment, err := deployment.CreateDeployment(gr.client, rbac.LocalCluster, namespace.Name, 2, "", "", false, false, false, true)
	require.NoError(gr.T(), err, "Failed to create deployment in the namespace")

	log.Infof("Verify the user %s can get the deployment.", createdUser.Username)
	userClient, err := gr.client.AsUser(createdUser)
	require.NoError(gr.T(), err)
	_, err = userClient.WranglerContext.Apps.Deployment().Get(namespace.Name, createdDeployment.Name, metav1.GetOptions{})
	require.NoError(gr.T(), err, "User failed to get the deployment")

	log.Infof("Verify the user %s cannot create a deployment.", createdUser.Username)
	_, err = deployment.CreateDeployment(userClient, rbac.LocalCluster, namespace.Name, 2, "", "", false, false, false, true)
	require.Error(gr.T(), err)
	require.True(gr.T(), errors.IsForbidden(err))

	log.Info("Delete global role.")
	err = rbacapi.DeleteGlobalRole(gr.client, createdGlobalRole.Name)
	require.NoError(gr.T(), err, "Failed to delete global role")
	_, err = rbac.GetGlobalRoleByName(gr.client, createdGlobalRole.Name)
	require.Error(gr.T(), err, "Global role was not deleted")

	log.Info("Verify the user cannot get the deployment.")
	_, err = userClient.WranglerContext.Apps.Deployment().Get(namespace.Name, createdDeployment.Name, metav1.GetOptions{})
	require.Error(gr.T(), err)
	require.True(gr.T(), errors.IsForbidden(err))
}

func (gr *GlobalRolesTestSuite) TestBuiltinGlobalRole() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a user with a builtin global role.")
	createdUser, err := createUserWithBuiltinRole(gr.client, rbac.Admin)
	require.NoError(gr.T(), err, "Failed to create user with builtin global role")

	log.Info("Verify that the global role binding is created for the user.")
	grb, err := rbac.GetGlobalRoleBindingByUserAndRole(gr.client, createdUser.ID, rbac.Admin.String())
	require.NoError(gr.T(), err, "Failed to get global role binding")
	require.NotEmpty(gr.T(), grb, "Global Role Binding not found for the user")

	log.Info("Verify the user can create a deployment.")
	_, namespace, err := projects.CreateProjectAndNamespace(gr.client, rbac.LocalCluster)
	require.NoError(gr.T(), err, "Failed to create project and namespace")
	userClient, err := gr.client.AsUser(createdUser)
	require.NoError(gr.T(), err)

	_, err = deployment.CreateDeployment(userClient, rbac.LocalCluster, namespace.Name, 2, "", "", false, false, false, true)
	require.NoError(gr.T(), err, "User failed to create deployment in the namespace")

	log.Info("Delete the global role binding for the user.")
	err = rbacapi.DeleteGlobalRoleBinding(gr.client, grb.Name)
	require.NoError(gr.T(), err, "Failed to delete global role binding")
	_, err = rbac.GetGlobalRoleBindingByName(gr.client, grb.Name)
	require.Error(gr.T(), err, "Global role binding was not deleted")

	log.Info("Verify the user cannot create a deployment.")
	_, err = deployment.CreateDeployment(userClient, rbac.LocalCluster, namespace.Name, 2, "", "", false, false, false, true)
	require.Error(gr.T(), err)
	require.True(gr.T(), errors.IsForbidden(err))
}

func (gr *GlobalRolesTestSuite) TestUpdateBuiltinGlobalRoleFails() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Attempt to update an existing built-in global role and verify it fails.")
	builtinGlobalRoleName := rbac.StandardUser.String()
	builtinGlobalRole, err := rbac.GetGlobalRoleByName(gr.client, builtinGlobalRoleName)
	require.NoError(gr.T(), err, "Failed to fetch the built-in global role")

	updatedGlobalRole := builtinGlobalRole.DeepCopy()
	updatedGlobalRole.Rules = append(updatedGlobalRole.Rules, rbacv1.PolicyRule{
		APIGroups: []string{"apps"},
		Resources: []string{"deployments"},
		Verbs:     []string{"create"},
	})

	_, err = rbacapi.UpdateGlobalRole(gr.client, updatedGlobalRole)
	require.Error(gr.T(), err, "Updating a built-in global role should fail")
	expectedErrMessage := fmt.Sprintf("%s updates to builtIn GlobalRoles for fields other than 'newUserDefault' are forbidden", webhookErrorMessagePrefix)
	require.Contains(gr.T(), err.Error(), expectedErrMessage)
}

func (gr *GlobalRolesTestSuite) TestDeleteBuiltinGlobalRoleFails() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Attempt to delete a built-in global role and verify it fails.")
	builtinGlobalRoleName := rbac.StandardUser.String()
	err := rbacapi.DeleteGlobalRole(gr.client, builtinGlobalRoleName)
	require.Error(gr.T(), err, "Deleting a built-in global role should fail")
	expectedErrMessage := fmt.Sprintf("%s cannot delete builtin GlobalRoles", webhookErrorMessagePrefix)
	require.Contains(gr.T(), err.Error(), expectedErrMessage)

	_, err = rbac.GetGlobalRoleByName(gr.client, builtinGlobalRoleName)
	require.NoError(gr.T(), err, "Failed to fetch the built-in global role")
}

func (gr *GlobalRolesTestSuite) TestConvertCustomGlobalRoleToBuiltinFails() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a custom global role.")
	createdGlobalRole, err := createCustomGlobalRole(gr.client)
	require.NoError(gr.T(), err, "Failed to create custom global role")

	log.Info("Attempt to convert a custom global role to a built-in global role and verify it fails.")
	convertedGlobalRole := createdGlobalRole.DeepCopy()
	convertedGlobalRole.Builtin = true
	_, err = rbacapi.UpdateGlobalRole(gr.client, convertedGlobalRole)
	require.Error(gr.T(), err, "Expected failure when updating to a built-in global role")
	expectedErrMessage := fmt.Sprintf("%s cannot update non-builtIn GlobalRole %s to be builtIn", webhookErrorMessagePrefix, createdGlobalRole.Name)
	require.Contains(gr.T(), err.Error(), expectedErrMessage)
}

func TestGlobalRolesTestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRolesTestSuite))
}
