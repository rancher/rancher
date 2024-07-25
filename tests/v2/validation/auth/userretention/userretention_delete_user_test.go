//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package userretention

import (
	"testing"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/v2/actions/auth"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	rbac1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type URDeleteTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (ur *URDeleteTestSuite) SetupSuite() {
	logrus.Info("Setting up URDeleteTestSuite")
	ur.session = session.NewSession()

	logrus.Info("Creating new Rancher client")
	client, err := rancher.NewClient("", ur.session)
	require.NoError(ur.T(), err)
	ur.client = client
}

func (ur *URDeleteTestSuite) TearDownSuite() {
	logrus.Info("Tearing down URDeleteTestSuite")
	ur.session.Cleanup()
}

func (ur *URDeleteTestSuite) TestDefaultAdminUserIsNotDeleted() {
	logrus.Info("Setting up user retention settings")
	setupUserRetentionSettings(ur.client, "", "400h", "*/1 * * * *", "false")

	logrus.Info("Getting admin user details")
	adminID, err := users.GetUserIDByName(ur.client, "admin")
	require.NoError(ur.T(), err)
	adminUser, err := ur.client.Management.User.ByID(adminID)
	require.NoError(ur.T(), err)
	adminUser.Password = ur.client.RancherConfig.AdminPassword

	logrus.Info("Updating user attributes")
	userAttributes, err := ur.client.WranglerContext.Mgmt.UserAttribute().Get(adminUser.ID, v1.GetOptions{})
	require.NoError(ur.T(), err)
	userAttributes.DeleteAfter = &v1.Duration{Duration: time.Second * 10}
	_, err = ur.client.WranglerContext.Mgmt.UserAttribute().Update(userAttributes)
	require.NoError(ur.T(), err)

	logrus.Info("Attempting initial login")
	_, err = auth.GetUserAfterLogin(ur.client, *adminUser)
	require.NoError(ur.T(), err)

	logrus.Info("Waiting for default duration")
	time.Sleep(defaultWaitDuration)

	logrus.Info("Attempting login after wait period")
	_, err = auth.GetUserAfterLogin(ur.client, *adminUser)
	require.NoError(ur.T(), err)
}

func (ur *URDeleteTestSuite) TestAdminUserGetDeleted() {
	logrus.Info("Setting up user retention settings")
	setupUserRetentionSettings(ur.client, "", "400h", "*/1 * * * *", "false")

	logrus.Info("Creating new admin user")
	newAdminUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "admin")
	require.NoError(ur.T(), err)
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newAdminUser)
	require.NoError(ur.T(), err)

	logrus.Info("Updating user attributes")
	userAttributes, err := ur.client.WranglerContext.Mgmt.UserAttribute().Get(newAdminUser.ID, v1.GetOptions{})
	require.NoError(ur.T(), err)
	userAttributes.DeleteAfter = &v1.Duration{Duration: time.Second * 10}
	_, err = ur.client.WranglerContext.Mgmt.UserAttribute().Update(userAttributes)
	require.NoError(ur.T(), err)

	logrus.Info("Verifying user status")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)
	require.NoError(ur.T(), err)

	logrus.Info("Waiting for default duration")
	pollUserStatus(ur.client, newAdminUser.ID, isInActive)

	logrus.Info("Attempting login after wait period")
	_, err = auth.GetUserAfterLogin(ur.client, *newAdminUser)
	assert.ErrorContains(ur.T(), err, "401 Unauthorized")

	logrus.Info("Checking bindings after user deletion")
	bindingsAfter, err := rbac.GetBindings(ur.client, newAdminUser.ID)
	require.NoError(ur.T(), err)
	assert.Empty(ur.T(), bindingsAfter["RoleBindings"], "Expected no RoleBindings after user deletion")
	assert.Equal(ur.T(), 0, len(bindingsAfter["RoleBindings"].([]rbac1.RoleBinding)), "RoleBindings slice should be empty after user deletion")

	logrus.Info("Checking global role bindings after user deletion")
	globalRoleBindingsAfter, err := ur.client.Management.GlobalRoleBinding.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"userId": newAdminUser.ID,
		},
	})
	require.NoError(ur.T(), err)
	assert.Empty(ur.T(), globalRoleBindingsAfter.Data, "Expected no GlobalRoleBindings after user deletion")
}

func (ur *URDeleteTestSuite) TestStandardUserGetDeleted() {
	logrus.Info("Setting up user retention settings")
	setupUserRetentionSettings(ur.client, "", "400h", "*/1 * * * *", "false")

	logrus.Info("Creating new standard user")
	newStdUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "user")
	require.NoError(ur.T(), err)

	logrus.Info("Logging in with new standard user")
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newStdUser)
	require.NoError(ur.T(), err)

	logrus.Info("Updating user attributes")
	userAttributes, err := ur.client.WranglerContext.Mgmt.UserAttribute().Get(newStdUser.ID, v1.GetOptions{})
	require.NoError(ur.T(), err)
	userAttributes.DeleteAfter = &v1.Duration{Duration: time.Second * 10}
	_, err = ur.client.WranglerContext.Mgmt.UserAttribute().Update(userAttributes)
	require.NoError(ur.T(), err)

	logrus.Info("Verifying user status")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)
	require.NoError(ur.T(), err)

	logrus.Info("Polling user status to disable")
	pollUserStatus(ur.client, newStdUser.ID, isInActive)

	logrus.Info("Attempting login after wait period")
	_, err = auth.GetUserAfterLogin(ur.client, *newStdUser)
	assert.ErrorContains(ur.T(), err, "401 Unauthorized")

	logrus.Info("Checking bindings after user deletion")
	bindingsAfter, err := rbac.GetBindings(ur.client, newStdUser.ID)
	require.NoError(ur.T(), err)
	assert.Empty(ur.T(), bindingsAfter["RoleBindings"], "Expected no RoleBindings after user deletion")
	assert.Equal(ur.T(), 0, len(bindingsAfter["RoleBindings"].([]rbac1.RoleBinding)), "RoleBindings slice should be empty after user deletion")

	logrus.Info("Checking global role bindings after user deletion")
	globalRoleBindingsAfter, err := ur.client.Management.GlobalRoleBinding.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"userId": newStdUser.ID,
		},
	})
	require.NoError(ur.T(), err)
	assert.Empty(ur.T(), globalRoleBindingsAfter.Data, "Expected no GlobalRoleBindings after user deletion")
}

func (ur *URDeleteTestSuite) TestUserIsNotDeletedWithBlankSettings() {
	logrus.Info("Setting up user retention settings with blank values")
	setupUserRetentionSettings(ur.client, "", "400h", "*/1 * * * *", "false")

	logrus.Info("Creating new standard user")
	newUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "user")
	require.NoError(ur.T(), err)

	logrus.Info("Logging in with new user")
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newUser)
	require.NoError(ur.T(), err)

	logrus.Info("Updating user attributes")
	userAttributes, err := ur.client.WranglerContext.Mgmt.UserAttribute().Get(newUser.ID, v1.GetOptions{})
	require.NoError(ur.T(), err)
	userAttributes.DeleteAfter = nil
	_, err = ur.client.WranglerContext.Mgmt.UserAttribute().Update(userAttributes)
	require.NoError(ur.T(), err)

	logrus.Info("Verifying user status")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)
	require.NoError(ur.T(), err)

	logrus.Info("Waiting for default duration")
	time.Sleep(defaultWaitDuration)

	logrus.Info("Verifying user status after wait period")
	userReLogin1, err := ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(userReLogin1).Enabled)
}

func (ur *URDeleteTestSuite) TestUserIsNotGetDeletedWithDryRun() {
	logrus.Info("Setting up user retention settings with dry run")
	setupUserRetentionSettings(ur.client, "", "400h", "*/1 * * * *", "true")

	logrus.Info("Creating new standard user")
	newUser, err := users.CreateUserWithRole(ur.client, users.UserConfig(), "user")
	require.NoError(ur.T(), err)

	logrus.Info("Logging in with new user")
	userReLogin, err := auth.GetUserAfterLogin(ur.client, *newUser)
	require.NoError(ur.T(), err)

	logrus.Info("Updating user attributes")
	userAttributes, err := ur.client.WranglerContext.Mgmt.UserAttribute().Get(newUser.ID, v1.GetOptions{})
	require.NoError(ur.T(), err)
	userAttributes.DeleteAfter = &v1.Duration{Duration: time.Second * 10}
	_, err = ur.client.WranglerContext.Mgmt.UserAttribute().Update(userAttributes)
	require.NoError(ur.T(), err)

	logrus.Info("Verifying user status")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)
	require.NoError(ur.T(), err)

	logrus.Info("Getting initial bindings")
	bindingsBefore, _ := rbac.GetBindings(ur.client, newUser.ID)

	logrus.Info("Waiting for default duration")
	time.Sleep(2 * defaultWaitDuration)

	logrus.Info("Verifying user status after wait period")
	userReLogin, err = ur.client.WranglerContext.Mgmt.User().Get(userReLogin.Name, v1.GetOptions{})
	require.NoError(ur.T(), err)
	assert.Equal(ur.T(), isActive, *(userReLogin).Enabled)

	logrus.Info("Checking bindings after wait period")
	bindingsAfter, _ := rbac.GetBindings(ur.client, newUser.ID)
	assert.Equal(ur.T(), bindingsBefore, bindingsAfter, "RoleBindings")
	assert.Equal(ur.T(), bindingsBefore, bindingsAfter, "ClusterRoleBindings")
	assert.Equal(ur.T(), bindingsBefore, bindingsAfter, "GlobalRoleBindings")
	assert.Equal(ur.T(), bindingsBefore, bindingsAfter, "ClusterRoleTemplateBindings")
	require.NoError(ur.T(), err)

	logrus.Info("Resetting user retention dry run setting")
	updateUserRetentionSettings(ur.client, userRetentionDryRun, "false")
}

func TestURDeleteUserSuite(t *testing.T) {
	suite.Run(t, new(URDeleteTestSuite))
}
