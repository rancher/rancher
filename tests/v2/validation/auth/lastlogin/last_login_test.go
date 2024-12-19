//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package lastlogin

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/auth"
	"github.com/rancher/shepherd/clients/rancher"
	users "github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LastLoginTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (ll *LastLoginTestSuite) TearDownSuite() {
	ll.session.Cleanup()
}

func (ll *LastLoginTestSuite) SetupSuite() {
	testSession := session.NewSession()
	ll.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(ll.T(), err)

	ll.client = client
}

func (ll *LastLoginTestSuite) TestLastLoginAdmin() {
	adminId, err := users.GetUserIDByName(ll.client, "admin")
	require.NoError(ll.T(), err)
	adminUser, err := ll.client.Management.User.ByID(adminId)
	adminUser.Password = ll.client.RancherConfig.AdminPassword

	adminRelogin, err := auth.GetUserAfterLogin(ll.client, *adminUser)
	require.NoError(ll.T(), err)

	lastLoginTime, err := auth.GetLastLoginTime(adminRelogin.Labels)
	require.NoError(ll.T(), err)
	require.False(ll.T(), lastLoginTime.IsZero(), "Last login is empty")

	userAttributes, err := ll.client.WranglerContext.Mgmt.UserAttribute().Get(adminId, v1.GetOptions{})
	assert.Equal(ll.T(), lastLoginTime, userAttributes.LastLogin.Time)
}

func (ll *LastLoginTestSuite) TestLastLoginStandardUser() {
	newUser, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser.Username)

	userLogin, err := auth.GetUserAfterLogin(ll.client, *newUser)
	require.NoError(ll.T(), err)

	lastLoginTime, err := auth.GetLastLoginTime(userLogin.Labels)
	require.NoError(ll.T(), err)
	require.False(ll.T(), lastLoginTime.IsZero(), "Last login is empty")

	userAttributes, err := ll.client.WranglerContext.Mgmt.UserAttribute().Get(newUser.ID, v1.GetOptions{})
	assert.Equal(ll.T(), lastLoginTime, userAttributes.LastLogin.Time)
}

func (ll *LastLoginTestSuite) TestReLoginUpdatesLastLogin() {
	newUser, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser.Username)

	userLogin, err := auth.GetUserAfterLogin(ll.client, *newUser)
	require.NoError(ll.T(), err)
	lastLoginTime, err := auth.GetLastLoginTime(userLogin.Labels)
	require.NoError(ll.T(), err)
	require.False(ll.T(), lastLoginTime.IsZero(), "Last login is empty")

	userAttributes, err := ll.client.WranglerContext.Mgmt.UserAttribute().Get(newUser.ID, v1.GetOptions{})
	assert.Equal(ll.T(), lastLoginTime, userAttributes.LastLogin.Time)

	userLogin, err = auth.GetUserAfterLogin(ll.client, *newUser)
	require.NoError(ll.T(), err)
	reLoginLastLoginTime, err := auth.GetLastLoginTime(userLogin.Labels)
	require.NoError(ll.T(), err)
	require.False(ll.T(), lastLoginTime.IsZero(), "Last login is empty")

	reLoginUserAttributes, err := ll.client.WranglerContext.Mgmt.UserAttribute().Get(newUser.ID, v1.GetOptions{})
	assert.Equal(ll.T(), reLoginLastLoginTime, reLoginUserAttributes.LastLogin.Time)

	if reLoginLastLoginTime.Before(lastLoginTime) || reLoginLastLoginTime.Equal(lastLoginTime) {
		require.FailNow(ll.T(), "Last login time after user relogs should be greater than initial lastlogin time.")
	}
}

func (ll *LastLoginTestSuite) TestUserLastLoginNotUpdated() {
	newUser, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser.Username)
	expectedErrMessage := "userattributes.management.cattle.io \"" + regexp.QuoteMeta(newUser.ID+"\" not found")

	user, err := ll.client.WranglerContext.Mgmt.User().Get(newUser.ID, v1.GetOptions{})

	_, exists := user.Labels[auth.LastloginLabel]
	if exists {
		ll.Assert().Fail("Expected the lastlogin to be not present and empty")
	}

	_, err = ll.client.WranglerContext.Mgmt.UserAttribute().Get(newUser.ID, v1.GetOptions{})
	require.Error(ll.T(), err)
	require.Contains(ll.T(), err.Error(), expectedErrMessage)
}

func (ll *LastLoginTestSuite) TestLastLoginNotUpdatedWhenLoginFails() {
	newUser, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser.Username)

	userLogin, err := auth.GetUserAfterLogin(ll.client, *newUser)
	require.NoError(ll.T(), err)
	lastLoginTimePreFailure, err := auth.GetLastLoginTime(userLogin.Labels)
	require.NoError(ll.T(), err)

	newUser.Password = password.GenerateUserPassword("testpass-")
	_, err = auth.GetUserAfterLogin(ll.client, *newUser)
	require.Error(ll.T(), err)
	k8sErrors.IsUnauthorized(err)

	userDetails, err := ll.client.WranglerContext.Mgmt.User().Get(newUser.ID, v1.GetOptions{})
	require.NoError(ll.T(), err)
	lastLoginTimePostFailure, err := auth.GetLastLoginTime(userDetails.Labels)
	require.NoError(ll.T(), err)
	userAttributes, err := ll.client.WranglerContext.Mgmt.UserAttribute().Get(newUser.ID, v1.GetOptions{})
	require.NoError(ll.T(), err)

	assert.Equal(ll.T(), lastLoginTimePostFailure, userAttributes.LastLogin.Time)
	require.Equal(ll.T(), lastLoginTimePreFailure, lastLoginTimePostFailure)
}

func (ll *LastLoginTestSuite) TestStandardUserCantUpdateLastLogin() {

	newUser, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser.Username)

	updatedUserLogin, err := auth.GetUserAfterLogin(ll.client, *newUser)
	require.NoError(ll.T(), err)

	lastLoginTime, err := auth.GetLastLoginTime(updatedUserLogin.Labels)
	require.NoError(ll.T(), err)
	require.False(ll.T(), lastLoginTime.IsZero(), "Last login is empty")

	lastLoginTime = lastLoginTime.Add(48 * time.Hour)
	newLastLogin := strconv.FormatInt(lastLoginTime.Unix(), 10)
	updatedUserLogin.Labels[auth.LastloginLabel] = newLastLogin

	standardClient, err := ll.client.AsUser(newUser)
	require.NoError(ll.T(), err)

	_, err = standardClient.WranglerContext.Mgmt.User().Update(updatedUserLogin)
	require.Error(ll.T(), err)
	k8sErrors.IsForbidden(err)

}

func (ll *LastLoginTestSuite) TestStandardUserCantDeleteLastLogin() {

	newUser, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser.Username)

	updatedUserLogin, err := auth.GetUserAfterLogin(ll.client, *newUser)
	require.NoError(ll.T(), err)

	_, exists := updatedUserLogin.Labels[auth.LastloginLabel]
	if !exists {
		ll.Assert().Fail("Expected the lastlogin to be not present and empty")
	}
	delete(updatedUserLogin.Labels, auth.LastloginLabel)

	standardClient, err := ll.client.AsUser(newUser)
	require.NoError(ll.T(), err)

	_, err = standardClient.WranglerContext.Mgmt.User().Update(updatedUserLogin)
	require.Error(ll.T(), err)
	k8sErrors.IsForbidden(err)
}

func (ll *LastLoginTestSuite) TestAdminCanDeleteLastLogin() {

	newUser, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "admin")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser.Username)

	updatedUserLogin, err := auth.GetUserAfterLogin(ll.client, *newUser)
	require.NoError(ll.T(), err)

	_, exists := updatedUserLogin.Labels[auth.LastloginLabel]
	if !exists {
		ll.Assert().Fail("Expected the lastlogin to be not present and empty")
	}
	delete(updatedUserLogin.Labels, auth.LastloginLabel)

	_, err = ll.client.WranglerContext.Mgmt.User().Update(updatedUserLogin)
	require.NoError(ll.T(), err)
}

func (ll *LastLoginTestSuite) TestConcurrentLoginAsMultipleUsers() {

	newUser1, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser1.Username)

	newUser2, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser2.Username)

	go func() {
		userLogin, err := auth.GetUserAfterLogin(ll.client, *newUser1)
		require.NoError(ll.T(), err)

		lastLoginTime, err := auth.GetLastLoginTime(userLogin.Labels)
		require.NoError(ll.T(), err)
		require.False(ll.T(), lastLoginTime.IsZero(), "Last login is empty")

		userAttributes, err := ll.client.WranglerContext.Mgmt.UserAttribute().Get(newUser1.ID, v1.GetOptions{})
		assert.Equal(ll.T(), lastLoginTime, userAttributes.LastLogin.Time)
	}()
	go func() {
		userLogin, err := auth.GetUserAfterLogin(ll.client, *newUser2)
		require.NoError(ll.T(), err)

		lastLoginTime, err := auth.GetLastLoginTime(userLogin.Labels)
		require.NoError(ll.T(), err)
		require.False(ll.T(), lastLoginTime.IsZero(), "Last login is empty")

		userAttributes, err := ll.client.WranglerContext.Mgmt.UserAttribute().Get(newUser2.ID, v1.GetOptions{})
		assert.Equal(ll.T(), lastLoginTime, userAttributes.LastLogin.Time)
	}()
}

func (ll *LastLoginTestSuite) TestConcurrentRefreshAndLogin() {

	newUser, err := users.CreateUserWithRole(ll.client, users.UserConfig(), "user")
	require.NoError(ll.T(), err)
	ll.T().Logf("Created a standard user: %v", newUser.Username)

	go func() {
		userLogin, err := auth.GetUserAfterLogin(ll.client, *newUser)
		require.NoError(ll.T(), err)

		lastLoginTime, err := auth.GetLastLoginTime(userLogin.Labels)
		require.NoError(ll.T(), err)
		require.False(ll.T(), lastLoginTime.IsZero(), "Last login is empty")

		userAttributes, err := ll.client.WranglerContext.Mgmt.UserAttribute().Get(newUser.ID, v1.GetOptions{})
		assert.Equal(ll.T(), lastLoginTime, userAttributes.LastLogin.Time)
	}()

	go func() {
		err = ll.client.Management.User.ActionRefreshauthprovideraccess(newUser)
		require.NoError(ll.T(), err)
	}()
}

func TestLastLoginTestSuite(t *testing.T) {
	suite.Run(t, new(LastLoginTestSuite))
}
