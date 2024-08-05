package provider

import (
	"slices"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/auth"
	v3 "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/namespaces"
	"github.com/rancher/shepherd/extensions/projects"
	"github.com/rancher/shepherd/extensions/rbac"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

type AuthProviderTestSuite struct {
	suite.Suite
	session    *session.Session
	client     *rancher.Client
	cluster    *clusters.ClusterMeta
	authConfig *AuthConfig
}

func (a *AuthProviderTestSuite) SetupSuite() {
	a.authConfig = new(AuthConfig)
	config.LoadConfig(ConfigurationFileKey, a.authConfig)

	if a.authConfig == nil {
		a.T().Skipf("Auth configuration is not provided, skipping the tests")
	}

	session := session.NewSession()
	a.session = session

	client, err := rancher.NewClient("", session)
	require.NoError(a.T(), err)

	a.client = client

	cluster, err := clusters.NewClusterMeta(a.client, a.client.RancherConfig.ClusterName)
	require.NoError(a.T(), err)

	a.cluster = cluster
}

func (a *AuthProviderTestSuite) TearDownSuite() {
	a.session.Cleanup()
}

func (a *AuthProviderTestSuite) TestAuth() {
	client := a.client

	tests := []struct {
		name         string
		authConfigID string
		enable       func(client *rancher.Client)
		update       func(existing, updates *v3.AuthConfig) (*v3.AuthConfig, error)
		admin        *v3.User
		authProvider auth.Provider
		searchBase   string
	}{
		{
			name:         "Open LDAP",
			authConfigID: "openldap",
			enable:       a.enableOLDAP,
			admin: &v3.User{
				Username: client.Auth.OLDAP.Config.Users.Admin.Username,
				Password: client.Auth.OLDAP.Config.Users.Admin.Password,
			},
			update:       client.Auth.OLDAP.Update,
			authProvider: auth.OpenLDAPAuth,
			searchBase:   client.Auth.OLDAP.Config.Users.SearchBase,
		},
	}

	for _, tt := range tests {
		session := session.NewSession()
		a.session = session

		client, err := client.WithSession(session)
		require.NoError(a.T(), err)

		defer session.Cleanup()

		logrus.Infof("Validating %v: Enabling", tt.name)
		tt.enable(client)

		authAdmin, err := login(a.client, tt.authProvider, tt.admin)
		require.NoError(a.T(), err, "Login as Admin failed")

		logrus.Infof("Validating %v: Access modes | Allow any valid user", tt.name)
		a.anyAccessMode(authAdmin, tt.authProvider)

		logrus.Infof("Validating %v: Refresh Group Memberships from the Users & Authentication", tt.name)
		a.refreshGroup(authAdmin, tt.authConfigID, a.authConfig.Group, a.authConfig.NestedGroup, tt.searchBase)

		logrus.Infof("Checking if the cluster name is given in the configuration for [%v] tests", tt.name)
		if a.client.RancherConfig.ClusterName == "" {
			a.T().Skipf("Cluster name is not provided, skipping the cluster for the %v test next steps", tt.name)
		}

		logrus.Infof("Validating %v: Group Membership", tt.name)
		a.groupMembership(authAdmin, tt.authProvider, tt.searchBase, tt.authConfigID)

		logrus.Infof("Validating %v: Access modes | Allow members of clusters and projects, plus authorized users & groups", tt.name)
		a.clusterAndProjectMembersAccessMode(authAdmin, tt.authProvider, tt.update, tt.searchBase, tt.authConfigID)

		logrus.Infof("Validating %v: Access modes | Restrict access to only the authorized users & groups", tt.name)
		a.restrictedAccessMode(authAdmin, tt.authProvider, tt.update, tt.searchBase, tt.authConfigID)
	}
}

func (a *AuthProviderTestSuite) anyAccessMode(authAdmin *rancher.Client, authProvider auth.Provider) {
	session := session.NewSession()
	defer session.Cleanup()

	authAdmin, err := authAdmin.WithSession(session)
	require.NoError(a.T(), err, "Instantiating as Admin with the new session failed")

	for _, v := range slices.Concat(a.authConfig.Users, a.authConfig.NestedUsers, a.authConfig.DoubleNestedUsers) {
		user := &v3.User{
			Username: v.Username,
			Password: v.Password,
		}

		logrus.Infof("Verifying logging as user [%v], should be able to login", v.Username)

		_, err := login(authAdmin, authProvider, user)
		require.NoError(a.T(), err)
	}
}

func (a *AuthProviderTestSuite) refreshGroup(authAdmin *rancher.Client, authConfigID, groupName, nestedGroupName, searchBase string) {
	session := session.NewSession()
	defer session.Cleanup()

	authAdmin, err := authAdmin.WithSession(session)
	require.NoError(a.T(), err, "Instantiating as Admin with the new session failed")

	adminGroupPrincipalID := newPrincipalID(authConfigID, "group", groupName, searchBase)
	newAdminGlobalRole := &v3.GlobalRoleBinding{
		GlobalRoleID:     rbac.Admin.String(),
		GroupPrincipalID: adminGroupPrincipalID,
	}

	logrus.Infof("Creating a global role binding for group [%v] with [%v] role", newAdminGlobalRole.GroupPrincipalID, newAdminGlobalRole.GlobalRoleID)

	_, err = authAdmin.Management.GlobalRoleBinding.Create(newAdminGlobalRole)
	require.NoError(a.T(), err, "Error occured while creating a role [%v]", newAdminGlobalRole)

	logrus.Infof("Verifying refreshing the group membership for group [%v]", groupName)

	err = users.RefreshGroupMembership(authAdmin)
	require.NoError(a.T(), err, "Error occured refreshing the group membership for group %v", groupName)

	standardGroupPrincipalID := newPrincipalID(authConfigID, "group", nestedGroupName, searchBase)
	newStandardGlobalRole := &v3.GlobalRoleBinding{
		GlobalRoleID:     rbac.StandardUser.String(),
		GroupPrincipalID: standardGroupPrincipalID,
	}

	logrus.Infof("Creating a global role binding for group [%v] with [%v] role", newStandardGlobalRole.GroupPrincipalID, newStandardGlobalRole.GlobalRoleID)

	_, err = authAdmin.Management.GlobalRoleBinding.Create(newStandardGlobalRole)
	require.NoError(a.T(), err, "Error occured while creating a role %v", newStandardGlobalRole)

	logrus.Infof("Verifying refreshing the group membership for group [%v]", nestedGroupName)

	err = users.RefreshGroupMembership(authAdmin)
	require.NoError(a.T(), err, "Error occured refreshing the group membership for group [%v]", nestedGroupName)
}

func (a *AuthProviderTestSuite) groupMembership(authAdmin *rancher.Client, authProvider auth.Provider, searchBase, authConfigID string) {
	session := session.NewSession()
	defer session.Cleanup()

	authAdmin, err := authAdmin.WithSession(session)
	require.NoError(a.T(), err, "Instantiating as Admin with the new session failed")

	doubleNestedGroupPrincipalID := newPrincipalID(authConfigID, "group", a.authConfig.DoubleNestedGroup, searchBase)
	groupCRTB := &v3.ClusterRoleTemplateBinding{
		ClusterID:        a.cluster.ID,
		GroupPrincipalID: doubleNestedGroupPrincipalID,
		RoleTemplateID:   rbac.ClusterOwner.String(),
	}
	logrus.Infof("Creating cluster role template binding for group [%v] with role [%v]", groupCRTB.GroupPrincipalID, groupCRTB.RoleTemplateID)
	_, err = authAdmin.Management.ClusterRoleTemplateBinding.Create(groupCRTB)
	require.NoError(a.T(), err, "Error occured while creating cluster role template binding")

	for _, v := range a.authConfig.DoubleNestedUsers {
		user := &v3.User{
			Username: v.Username,
			Password: v.Password,
		}
		userClient, err := login(authAdmin, authProvider, user)
		require.NoError(a.T(), err)

		newUserClient, err := userClient.ReLogin()
		require.NoError(a.T(), err)

		clusterList, err := newUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).List(nil)
		require.NoError(a.T(), err)

		logrus.Infof("Verifying user [%v] lists [%v] clusters while expecting [%v] clusters to be listed", v.Username, len(clusterList.Data), 1)
		assert.Equalf(a.T(), 1, len(clusterList.Data), "Error occured while: user [%v] lists [%v] clusters while expecting [%v] clusters to be listed", v.Username, len(clusterList.Data), 1)
	}

	for _, v := range slices.Concat(a.authConfig.Users, a.authConfig.NestedUsers) {
		user := &v3.User{
			Username: v.Username,
			Password: v.Password,
		}
		userClient, err := login(authAdmin, authProvider, user)
		require.NoError(a.T(), err)

		logrus.Infof("Verifying user [%v] should NOT lists clusters", v.Username)

		_, err = userClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).List(nil)

		assert.NotNilf(a.T(), err, "Error should contain error message", "Error occured while: user [%v] should NOT lists clusters", v.Username)
		assert.Containsf(a.T(), err.Error(), "Resource type [provisioning.cattle.io.cluster] has no method GET", "Error occured while: user [%v] should NOT lists clusters", v.Username)
	}

	var createdCRTB *rbacv1.ClusterRoleBinding

	listCRTB, err := authAdmin.Steve.SteveType("rbac.authorization.k8s.io.clusterrolebinding").List(nil)
	require.NoError(a.T(), err)

	for _, v := range listCRTB.Data {
		crtb := &rbacv1.ClusterRoleBinding{}

		err = v1.ConvertToK8sType(v.JSONResp, crtb)
		require.NoError(a.T(), err)

		for _, v := range crtb.Subjects {
			if v.Name == doubleNestedGroupPrincipalID {
				createdCRTB = crtb
			}
		}
	}
	assert.NotNilf(a.T(), createdCRTB, "Error occured while: creating the CRTB")

	logrus.Infof("Creating a project with the admin")

	projectTemplate := projects.NewProjectConfig(a.cluster.ID)
	projectResp, err := authAdmin.Management.Project.Create(projectTemplate)

	require.NoError(a.T(), err)
	assert.NotNilf(a.T(), projectResp, "Error occured while: project is created with the admin")

	nestedGroupPrincipalID := newPrincipalID(authConfigID, "group", a.authConfig.NestedGroup, searchBase)
	groupPRTB := &v3.ProjectRoleTemplateBinding{
		ProjectID:        projectResp.ID,
		GroupPrincipalID: nestedGroupPrincipalID,
		RoleTemplateID:   rbac.ProjectOwner.String(),
	}

	logrus.Infof("Creating a PRTB for group [%v]", a.authConfig.NestedGroup)

	groupPRTBResp, err := authAdmin.Management.ProjectRoleTemplateBinding.Create(groupPRTB)
	require.NoError(a.T(), err)
	assert.NotNilf(a.T(), groupPRTBResp, "Error occured while: creating a  PRTB for group [%v]", a.authConfig.NestedGroup)

	for _, v := range a.authConfig.NestedUsers {
		user := &v3.User{
			Username: v.Username,
			Password: v.Password,
		}
		userClient, err := login(authAdmin, authProvider, user)
		require.NoError(a.T(), err)

		namespaceName := namegen.AppendRandomString("testns-")
		namespace, err := namespaces.CreateNamespace(userClient, namespaceName, "{}", nil, nil, projectResp)
		require.NoError(a.T(), err)

		logrus.Infof("Verifying user [%v] has created namespace [%v]", v.Username, namespaceName)
		assert.Equal(a.T(), namespaceName, namespace.Name, "Error occured while: user [%v] has created namespace [%v]", v.Username, namespaceName)
	}
}

func (a *AuthProviderTestSuite) restrictedAccessMode(
	authAdmin *rancher.Client, authProvider auth.Provider, update func(existing, updates *v3.AuthConfig) (*v3.AuthConfig, error), searchBase string, authConfigID string,
) {
	session := session.NewSession()
	defer session.Cleanup()

	authAdmin, err := authAdmin.WithSession(session)
	require.NoError(a.T(), err, "Instantiating as Admin with the new session failed")

	groupPrincipalID := newPrincipalID(authConfigID, "group", a.authConfig.Group, searchBase)
	groupCRTB := &v3.ClusterRoleTemplateBinding{
		ClusterID:        a.cluster.ID,
		GroupPrincipalID: groupPrincipalID,
		RoleTemplateID:   rbac.ClusterMember.String(),
	}

	logrus.Infof("Creating cluster role template binding for group [%v] with role [%v]", groupCRTB.GroupPrincipalID, groupCRTB.RoleTemplateID)

	_, err = authAdmin.Management.ClusterRoleTemplateBinding.Create(groupCRTB)
	require.NoError(a.T(), err, "Error occured while creating cluster role template binding")

	defaultProject, err := projects.GetProjectByName(authAdmin, a.cluster.ID, "Default")
	require.NoError(a.T(), err)

	for _, v := range a.authConfig.NestedUsers {
		nestedUserPrincipalID := newPrincipalID(authConfigID, "user", v.Username, searchBase)

		userPRTB := &v3.ProjectRoleTemplateBinding{
			ProjectID:        defaultProject.ID,
			GroupPrincipalID: nestedUserPrincipalID,
			RoleTemplateID:   rbac.ProjectOwner.String(),
		}

		logrus.Infof("Creating project role template binding for user [%v] with role [%v]", userPRTB.GroupPrincipalID, userPRTB.RoleTemplateID)
		userPRTBResp, err := authAdmin.Management.ProjectRoleTemplateBinding.Create(userPRTB)
		require.NoError(a.T(), err)

		logrus.Infof("Verifying project role template binding has created for user [%v]", v.Username)
		assert.NotNilf(a.T(), userPRTBResp, "Error occured while: project role template binding is created for user [%v]", v.Username)
	}

	var principalIDs []string

	principalIDs = append(principalIDs, newPrincipalID(authConfigID, "group", a.authConfig.DoubleNestedGroup, searchBase))
	for _, v := range a.authConfig.DoubleNestedUsers {
		principalIDs = append(principalIDs, newPrincipalID(authConfigID, "user", v.Username, searchBase))
	}

	logrus.Infof("Verifying access mode updated to restrict access to only the authorized users & groups")

	existing, updates := newWithAccessMode(a.T(), authAdmin, authConfigID, "required", principalIDs)
	newAuthConfig, err := update(existing, updates)

	require.NoError(a.T(), err)
	assert.Equal(a.T(), existing.AccessMode, newAuthConfig.AccessMode, "Error occured while: access mode updated to restrict access to only the authorized users & groups")

	for _, v := range a.authConfig.DoubleNestedUsers {
		user := &v3.User{
			Username: v.Username,
			Password: v.Password,
		}
		_, err := login(authAdmin, authProvider, user)

		logrus.Infof("Verifying logging as user [%v], should be able to login", v.Username)
		assert.NoErrorf(a.T(), err, "Error occured while: logging as user [%v], should be able to login", v.Username)

	}

	for _, v := range slices.Concat(a.authConfig.Users, a.authConfig.NestedUsers) {
		user := &v3.User{
			Username: v.Username,
			Password: v.Password,
		}
		_, err := login(authAdmin, authProvider, user)

		logrus.Infof("Verifying logging as user [%v], should be able to login", v.Username)
		assert.Errorf(a.T(), err, "Error occured while: logging as user [%v], should be able to login", v.Username)
	}

	logrus.Infof("Rolling back the access mode to unrestricted from restrict access to only the authorized users & groups")

	// Rollback only happens with the local client change a.client to authAdmin to reproduce the issue
	authExisting, authWithUnrestricted := newWithAccessMode(a.T(), a.client, authConfigID, "unrestricted", nil)
	newAuthConfig, err = update(authExisting, authWithUnrestricted)

	require.NoError(a.T(), err)
	assert.Equal(a.T(), authExisting.AccessMode, newAuthConfig.AccessMode, "Rolling back the access mode to unrestricted from restrict access to only the authorized users & groups")
}

func (a *AuthProviderTestSuite) clusterAndProjectMembersAccessMode(
	authAdmin *rancher.Client,
	authProvider auth.Provider,
	update func(existing, updates *v3.AuthConfig) (*v3.AuthConfig, error),
	searchBase string, authConfigID string,
) {
	session := session.NewSession()
	defer session.Cleanup()

	authAdmin, err := authAdmin.WithSession(session)
	require.NoError(a.T(), err, "Instantiating as Admin with the new session failed")

	doubleNestedGroupPrincipalID := newPrincipalID(authConfigID, "group", a.authConfig.DoubleNestedGroup, searchBase)
	groupCRTB := &v3.ClusterRoleTemplateBinding{
		ClusterID:        a.cluster.ID,
		GroupPrincipalID: doubleNestedGroupPrincipalID,
		RoleTemplateID:   rbac.ClusterMember.String(),
	}

	logrus.Infof("Creating cluster role template binding for group [%v] with role [%v]", groupCRTB.GroupPrincipalID, groupCRTB.RoleTemplateID)
	_, err = authAdmin.Management.ClusterRoleTemplateBinding.Create(groupCRTB)
	require.NoError(a.T(), err, "Error occured while creating cluster role template binding")

	defaultProject, err := projects.GetProjectByName(authAdmin, a.cluster.ID, "Default")
	require.NoError(a.T(), err)

	for _, v := range a.authConfig.NestedUsers {
		nestedGroupPrincipalID := newPrincipalID(authConfigID, "user", v.Username, searchBase)

		userPRTB := &v3.ProjectRoleTemplateBinding{
			ProjectID:        defaultProject.ID,
			GroupPrincipalID: nestedGroupPrincipalID,
			RoleTemplateID:   rbac.ProjectOwner.String(),
		}
		userPRTBResp, err := authAdmin.Management.ProjectRoleTemplateBinding.Create(userPRTB)
		require.NoError(a.T(), err)

		logrus.Infof("Verifying project role binding has created for user [%v]", v.Username)
		assert.NotNilf(a.T(), userPRTBResp, "Error occured while: creating a project role binding for user [%v]", v.Username)
	}

	logrus.Infof("Verifying access mode updated to allow members of clusters and projects, plus authorized users & groups")

	authExisting, authWithRestricted := newWithAccessMode(a.T(), authAdmin, authConfigID, "restricted", nil)
	newAuthConfig, err := update(authExisting, authWithRestricted)

	require.NoError(a.T(), err)
	assert.Equal(a.T(), authExisting.AccessMode, newAuthConfig.AccessMode, "Error occured while: access mode updated to allow members of clusters and projects, plus authorized users & groups")

	for _, v := range slices.Concat(a.authConfig.DoubleNestedUsers, a.authConfig.NestedUsers) {
		user := &v3.User{
			Username: v.Username,
			Password: v.Password,
		}
		_, err := login(authAdmin, authProvider, user)

		logrus.Infof("Verifying logging as user [%v], should be able to login", v.Username)
		assert.NoErrorf(a.T(), err, "Error occured while: logging as user [%v], should be able to login", v.Username)
	}

	for _, v := range a.authConfig.Users {
		user := &v3.User{
			Username: v.Username,
			Password: v.Password,
		}
		_, err := login(authAdmin, authProvider, user)

		logrus.Infof("Verifying logging as user [%v], should NOT be able to login", v.Username)
		assert.Errorf(a.T(), err, "Verifying logging as user [%v], should NOT be able to login", v.Username)
	}

	logrus.Infof("Rolling back the access mode to unrestricted from allow members of clusters and projects, plus authorized users & groups")

	authExisting, authWithUnrestricted := newWithAccessMode(a.T(), authAdmin, authConfigID, "unrestricted", nil)
	newAuthConfig, err = update(authExisting, authWithUnrestricted)

	require.NoError(a.T(), err)
	assert.Equal(a.T(), authExisting.AccessMode, newAuthConfig.AccessMode, "Rolling back the access mode to unrestricted from allow members of clusters and projects, plus authorized users & groups")
}

func (a *AuthProviderTestSuite) enableOLDAP(client *rancher.Client) {
	err := client.Auth.OLDAP.Enable()
	require.NoError(a.T(), err)

	ldapConfig, err := client.Management.AuthConfig.ByID("openldap")
	require.NoError(a.T(), err)

	assert.Truef(a.T(), ldapConfig.Enabled, "Checking if Open LDAP has enabled")

	assert.Equalf(a.T(), authProvCleanupAnnotationValUnlocked, ldapConfig.Annotations[authProvCleanupAnnotationKey], "Checking if annotation set to unlocked for LDAP Auth Config")

	passwordSecretResp, err := client.Steve.SteveType("secret").ByID(passwordSecretID)
	assert.NoErrorf(a.T(), err, "Checking open LDAP config secret for service account password exists")

	passwordSecret := &corev1.Secret{}
	err = v1.ConvertToK8sType(passwordSecretResp.JSONResp, passwordSecret)
	require.NoError(a.T(), err)

	assert.Equal(a.T(), client.Auth.OLDAP.Config.ServiceAccount.Password, string(passwordSecret.Data["serviceaccountpassword"]), "Checking if serviceaccountpassword value is equal to the given")
}

func TestAuthProviderSuite(t *testing.T) {
	suite.Run(t, new(AuthProviderTestSuite))
}
