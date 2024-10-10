//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package userretention

import (
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/auth"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	managementv3 "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type URDisableTestSuite struct {
	suite.Suite
	client      *rancher.Client
	session     *session.Session
	adminClient *rancher.Client
	adminUser   *managementv3.User
}

func (ur *URDisableTestSuite) SetupSuite() {
	logrus.Info("Setting up URDisableTestSuite")
	ur.session = session.NewSession()

	logrus.Info("Creating new Rancher client")
	client, err := rancher.NewClient("", ur.session)
	require.NoError(ur.T(), err)
	ur.client = client
}

func (ur *URDisableTestSuite) TearDownSuite() {
	logrus.Info("Tearing down URDisableTestSuite")
	if ur.adminUser != nil {
		logrus.Info("Deleting admin user")
		err := ur.client.Management.User.Delete(ur.adminUser)
		if err != nil {
			logrus.Errorf("Failed to delete admin user: %v", err)
		}
	}
	logrus.Info("Cleaning up session")
	ur.session.Cleanup()
}

func (ur *URDisableTestSuite) assertBindingsEqual(before, after map[string]interface{}) {
	assert.Equal(ur.T(), before["RoleBindings"], after["RoleBindings"], "RoleBindings mismatch")
	assert.Equal(ur.T(), before["ClusterRoleBindings"], after["ClusterRoleBindings"], "ClusterRoleBindings mismatch")
	assert.Equal(ur.T(), before["GlobalRoleBindings"], after["GlobalRoleBindings"], "GlobalRoleBindings mismatch")
	assert.Equal(ur.T(), before["ClusterRoleTemplateBindings"], after["ClusterRoleTemplateBindings"], "ClusterRoleTemplateBindings mismatch")
}

func (ur *URDisableTestSuite) TestDefaultAdminUserIsNotDisabled() {
	logrus.Info("Setting up user retention settings")
	setupUserRetentionSettings(ur.client, "10s", "", "*/1 * * * *", "false")
	logrus.Info("Retrieving admin user details")
	adminID, err := users.GetUserIDByName(ur.client, "admin")
	require.NoError(ur.T(), err)
	adminUser, err := ur.client.Management.User.ByID(adminID)
	require.NoError(ur.T(), err)
	adminUser.Password = ur.client.RancherConfig.AdminPassword

	logrus.Info("Attempting initial login for admin user")
	_, err = auth.GetUserAfterLogin(ur.client, *adminUser)
	require.NoError(ur.T(), err)

	logrus.Info("Waiting for default duration")
	time.Sleep(defaultWaitDuration)

	logrus.Info("Attempting login after wait period")
	_, err = auth.GetUserAfterLogin(ur.client, *adminUser)
	require.NoError(ur.T(), err)
}

func (ur *URDisableTestSuite) TestAdminUserGetDisabled() {
	logrus.Info("Setting up user retention settings")
	setupUserRetentionSettings(ur.client, "10s", "", "*/1 * * * *", "false")

	logrus.Info("Creating new admin user")
	newAdminUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "admin")
	require.NoError(ur.T(), err)
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newAdminUser)
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)

	bindingsBefore, err := rbac.GetBindings(ur.client, newAdminUser.ID)
	require.NoError(ur.T(), err, "Failed to get initial bindings")

	logrus.Info("Polling user status")
	pollUserStatus(ur.client, newAdminUser.ID, isInActive)

	logrus.Info("Verifying user status after polling")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isInActive, *(userReLogin).Enabled)

	logrus.Info("Attempting login with disabled user")
	_, err = auth.GetUserAfterLogin(ur.client, *newAdminUser)
	assert.ErrorContains(ur.T(), err, "403 Forbidden")
	bindingsAfter, err := rbac.GetBindings(ur.client, newAdminUser.ID)
	require.NoError(ur.T(), err, "Failed to get final bindings")
	ur.assertBindingsEqual(bindingsBefore, bindingsAfter)

}

func (ur *URDisableTestSuite) TestStandardUserGetDisabled() {
	logrus.Info("Setting up user retention settings")
	setupUserRetentionSettings(ur.client, "10s", "", "*/1 * * * *", "false")

	logrus.Info("Creating new standard user")
	newStdUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "user")
	require.NoError(ur.T(), err)
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newStdUser)
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)

	logrus.Info("Getting initial bindings")
	bindingsBefore, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err, "Failed to get initial bindings")

	logrus.Info("Polling user status")
	pollUserStatus(ur.client, newStdUser.ID, isInActive)

	logrus.Info("Verifying user status after polling")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isInActive, *(userReLogin).Enabled)

	logrus.Info("Attempting login with disabled user")
	_, err = auth.GetUserAfterLogin(ur.client, *newStdUser)
	assert.ErrorContains(ur.T(), err, "403 Forbidden")

	logrus.Info("Getting final bindings")
	bindingsAfter, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err, "Failed to get final bindings")
	ur.assertBindingsEqual(bindingsBefore, bindingsAfter)
}

func (ur *URDisableTestSuite) TestDisabledUserGetEnabled() {
	logrus.Info("Setting up user retention settings")
	setupUserRetentionSettings(ur.client, "10s", "", "*/1 * * * *", "false")

	logrus.Info("Creating new standard user")
	newStdUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "user")
	require.NoError(ur.T(), err)
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newStdUser)
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)

	logrus.Info("Getting initial bindings")
	bindingsBefore, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err, "Failed to get initial bindings")

	logrus.Info("Polling user status to disable")
	pollUserStatus(ur.client, newStdUser.ID, isInActive)

	logrus.Info("Verifying user status after polling")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isInActive, *(userReLogin).Enabled)

	logrus.Info("Attempting login with disabled user")
	_, err = auth.GetUserAfterLogin(ur.client, *newStdUser)
	assert.ErrorContains(ur.T(), err, "403 Forbidden")

	logrus.Info("Enabling the disabled user")
	enabled := true
	userReLogin.Enabled = &enabled
	activateUser, err := ur.client.WranglerContext.Mgmt.User().Update(userReLogin)
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(activateUser).Enabled)

	logrus.Info("Verifying user status after enabling")
	activeUserReLogin, err := ur.client.WranglerContext.Mgmt.User().Get(activateUser.Name, v1.GetOptions{})
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(activeUserReLogin).Enabled)

	logrus.Info("Getting final bindings")
	bindingsAfterEnabled, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err, "Failed to get final bindings")
	ur.assertBindingsEqual(bindingsBefore, bindingsAfterEnabled)
}

func (ur *URDisableTestSuite) TestStandardUserDidNotGetDisabledWithBlankSettings() {
	logrus.Info("Setting up user retention settings with blank values")
	setupUserRetentionSettings(ur.client, "", "", "*/1 * * * *", "false")

	logrus.Info("Creating new standard user")
	newUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "user")
	require.NoError(ur.T(), err)
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newUser)
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)

	logrus.Info("Waiting for default duration")
	time.Sleep(defaultWaitDuration)

	logrus.Info("Attempting login after wait period")
	userReLogin, err = auth.GetUserAfterLogin(ur.client, *newUser)
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)
}

func (ur *URDisableTestSuite) TestUserDisableByUpdatingUserattributes() {
	logrus.Info("Setting up user retention settings")
	setupUserRetentionSettings(ur.client, "10s", "", "*/1 * * * *", "false")

	logrus.Info("Creating new standard user")
	newStdUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "user")
	require.NoError(ur.T(), err)
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newStdUser)
	require.NoError(ur.T(), err)

	logrus.Info("Getting initial bindings")
	bindingsBefore, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err, "Failed to get initial bindings")

	logrus.Info("Updating user attributes")
	userAttributes, err := ur.client.WranglerContext.Mgmt.UserAttribute().Get(newStdUser.ID, v1.GetOptions{})
	require.NoError(ur.T(), err)
	userAttributes.DisableAfter = &v1.Duration{Duration: time.Second * 10}
	_, err = ur.client.WranglerContext.Mgmt.UserAttribute().Update(userAttributes)
	require.NoError(ur.T(), err)

	logrus.Info("Verifying user status after updating attributes")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)
	require.NoError(ur.T(), err)

	logrus.Info("Polling user status")
	pollUserStatus(ur.client, newStdUser.ID, isInActive)

	logrus.Info("Verifying user status after polling")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isInActive, *(userReLogin).Enabled)

	logrus.Info("Attempting login with disabled user")
	_, err = auth.GetUserAfterLogin(ur.client, *newStdUser)
	assert.ErrorContains(ur.T(), err, "403 Forbidden")

	logrus.Info("Getting final bindings")
	bindingsAfter, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err, "Failed to get final bindings")
	ur.assertBindingsEqual(bindingsBefore, bindingsAfter)
}

func (ur *URDisableTestSuite) TestUserIsNotDisabledWithDryRun() {
	logrus.Info("Setting up user retention settings with dry run")
	setupUserRetentionSettings(ur.client, "10s", "", "*/1 * * * *", "true")

	logrus.Info("Creating new standard user")
	newStdUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "user")
	require.NoError(ur.T(), err)
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newStdUser)
	require.NoError(ur.T(), err)

	logrus.Info("Getting initial bindings")
	bindingsBefore, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err, "Failed to get initial bindings")

	logrus.Info("Updating user attributes")
	userAttributes, err := ur.client.WranglerContext.Mgmt.UserAttribute().Get(newStdUser.ID, v1.GetOptions{})
	require.NoError(ur.T(), err)
	userAttributes.DisableAfter = &v1.Duration{Duration: time.Second * 10}
	_, err = ur.client.WranglerContext.Mgmt.UserAttribute().Update(userAttributes)
	require.NoError(ur.T(), err)

	logrus.Info("Verifying user status after updating attributes")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)
	require.NoError(ur.T(), err)

	logrus.Info("Waiting for default duration")
	time.Sleep(2 * defaultWaitDuration)

	logrus.Info("Attempting login after wait period")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)

	logrus.Info("Getting final bindings")
	bindingsAfter, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err, "Failed to get final bindings")
	ur.assertBindingsEqual(bindingsBefore, bindingsAfter)

	logrus.Info("User retention settings:dry run settings back to default value")
	updateUserRetentionSettings(ur.client, userRetentionDryRun, "false")
}

func TestURDisableUserSuite(t *testing.T) {
	suite.Run(t, new(URDisableTestSuite))
}
