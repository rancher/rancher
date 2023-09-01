package openldapauth

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOpenLDAPAuthConfig(t *testing.T) {
	rancherClient, err := rancher.NewRancherClient()
	require.NoError(t, err)

	authConfig, err := CreateOpenLDAPAuthConfig(rancherClient)
	require.NoError(t, err)

	assert.Equal(t, openLdapConfigNameBase, authConfig.Name)
	assert.NotNil(t, authConfig.OpenLDAPCredentialConfig)
	assert.Equal(t, []string{"openldapqa.qa.rancher.space"}, authConfig.OpenLDAPCredentialConfig.Servers)
	assert.Equal(t, "cn=admin,dc=qa,rancher,dc=space", authConfig.OpenLDAPCredentialConfig.ServiceAccountDistinguishedName)

}
