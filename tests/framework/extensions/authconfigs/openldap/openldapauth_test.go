package openldapauth

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type openLdapTest struct {
	suite.Suite
	testUser *management.User
	client   *rancher.Client
	project  *management.Project
	session  *session.Session
}

func (d *openLdapTest) SetupSuite() {
	testSession := session.NewSession(d.T())

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	d.client = client

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
	}

	testProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(p.T(), err)

	d.project = testProject

	enabled := true

	user := &management.User{
		Username: "testusername",
		Password: "passwordpasswordd",
		Name:     "displayname",
		Enabled:  &enabled,
	}

}
func (d *openLdapTest) TearDownSuite() {
	d.session.Cleanup()
}

func TestCreateOpenLDAPAuthConfig(t *testing.T) {
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

//need to add a suite a teardown and a config = somewehre?
