package auth

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/auth"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	corev1 "k8s.io/api/core/v1"
)

type OLDAPTestSuite struct {
	suite.Suite
	session *session.Session
	client  *rancher.Client
}

func (o *OLDAPTestSuite) TearDownSuite() {
	o.session.Cleanup()
}

func (o *OLDAPTestSuite) SetupSuite() {
	testSession := session.NewSession()
	o.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(o.T(), err)

	o.client = client
}

func (o *OLDAPTestSuite) TestEnableOLDAP() {
	subSession := o.session.NewSession()
	defer subSession.Cleanup()

	client, err := o.client.WithSession(subSession)
	require.NoError(o.T(), err)

	a, err := auth.NewAuth(client, subSession)
	require.NoError(o.T(), err)

	err = a.OLDAP.Enable()
	require.NoError(o.T(), err)

	ldapConfig, err := client.Management.AuthConfig.ByID("openldap")
	require.NoError(o.T(), err)

	assert.Truef(o.T(), ldapConfig.Enabled, "Checking if Open LDAP is enabled")

	assert.Equalf(o.T(), authProvCleanupAnnotationValUnlocked, ldapConfig.Annotations[authProvCleanupAnnotationKey], "Checking if annotation set to unlocked for LDAP Auth Config")

	passwordSecretResp, err := client.Steve.SteveType("secret").ByID(passwordSecretID)
	assert.NoErrorf(o.T(), err, "Checking open LDAP config secret for service account password exists")

	passwordSecret := &corev1.Secret{}
	err = v1.ConvertToK8sType(passwordSecretResp.JSONResp, passwordSecret)
	require.NoError(o.T(), err)

	assert.Equal(o.T(), a.OLDAP.Config.ServiceAccount.Password, string(passwordSecret.Data["serviceaccountpassword"]), "Checking if serviceaccountpassword value is equal to the given")
}

func (o *OLDAPTestSuite) TestDisableOLDAP() {
	subSession := o.session.NewSession()
	defer subSession.Cleanup()

	client, err := o.client.WithSession(subSession)
	require.NoError(o.T(), err)

	a, err := auth.NewAuth(client, subSession)
	require.NoError(o.T(), err)

	err = a.OLDAP.Disable()
	require.NoError(o.T(), err)

	ldapConfig, err := waitUntilAnnotationIsUpdated(client)
	require.NoError(o.T(), err)

	assert.Falsef(o.T(), ldapConfig.Enabled, "Checking if Open LDAP is disabled")

	assert.Equalf(o.T(), authProvCleanupAnnotationValLocked, ldapConfig.Annotations[authProvCleanupAnnotationKey], "Checking if annotation set to locked for LDAP Auth Config")

	_, err = client.Steve.SteveType("secret").ByID(passwordSecretID)
	assert.Errorf(o.T(), err, "Checking open LDAP config secret for service account password does not exist")
	assert.Containsf(o.T(), err.Error(), "404", "Checking open LDAP config secret for service account password error returns 404")
}

func TestOLDAPSuite(t *testing.T) {
	suite.Run(t, new(OLDAPTestSuite))
}
