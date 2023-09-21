package openldapauth

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OpenLdapTest struct {
	suite.Suite
	testUser *management.User
	client   *rancher.Client
	project  *management.Project
	session  *session.Session
}

func (d *OpenLdapTest) SetupSuite() {
	testSession := session.NewSession()

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	d.client = client

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
	}

	testProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(d.T(), err)

	d.project = testProject
	/*
		upgradeConfig := new(Config2)
		config.LoadConfig(ConfigurationFileKey, upgradeConfig)
		fmt.Print(upgradeConfig.OpenLdapUser)
		fmt.Print(upgradeConfig.OpenLdapUserPass) */

	//enabled := true

	/*user := &management.User{
		Username: "testusername",
		Password: "passwordpasswordd",
		Name:     "displayname",
		Enabled:  &enabled,
	}*/

	//client.AsUser(user)

}
func (d *OpenLdapTest) TearDownSuite() {
	d.session.Cleanup()
}

func (d *OpenLdapTest) TestEnableOpenLDAP() {
	testSession := session.NewSession()
	d.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	upgradeConfig := new(Config2)
	config.LoadConfig(ConfigurationFileKey, upgradeConfig)
	fmt.Print(upgradeConfig.OpenLdapUser)
	fmt.Print(upgradeConfig.OpenLdapUserPass)
	fmt.Print(client)
	fmt.Print(d.testUser)

	/*
		rancherClient, err := rancher.NewRancherClient()
		require.NoError(t, err)

		authConfig, err := CreateOpenLDAPAuthConfig(rancherClient)
		require.NoError(t, err)
	*/
	/*
		assert.Equal(t, openLdapConfigNameBase, authConfig.Name)
		assert.NotNil(t, authConfig.OpenLDAPCredentialConfig)
		assert.Equal(t, []string{"openldapqa.qa.rancher.space"}, authConfig.OpenLDAPCredentialConfig.Servers)
		assert.Equal(t, "cn=admin,dc=qa,rancher,dc=space", authConfig.OpenLDAPCredentialConfig.ServiceAccountDistinguishedName)
	*/
}

func TestOpenLdapTestSuite(t *testing.T) {
	suite.Run(t, new(OpenLdapTest))
}
