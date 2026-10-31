package integration

import (
	"context"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/cognito"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	"github.com/rancher/rancher/pkg/auth/providers/googleoauth"
	"github.com/rancher/rancher/pkg/auth/providers/keycloakoidc"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AuthConfigTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *AuthConfigTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *AuthConfigTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

// This is commented out because we can't CURRENTLY create AuthConfig Resources

// TestAuthConfigActions verifies that each auth config type exposes the
// expected set of actions (testAndApply, configureTest, testAndEnable).
func (s *AuthConfigTestSuite) TestAuthConfigActions() {
	s.T().Skip("Unable to create AuthConfigs with the client")
	for name, config := range authProviderTypes {
		createAuthConfig(s.T(), s.client.Management.AuthConfig, name, config)
	}

	configs, err := s.client.Management.AuthConfig.List(nil)
	s.Require().NoError(err)

	configMap := map[string]management.AuthConfig{}
	for _, config := range configs.Data {
		configMap[config.Type] = config
	}

	// Configs that should have testAndApply action.
	testAndApplyConfigs := []string{
		"activeDirectoryConfig",
		"azureADConfig",
		"cognitoConfig",
		"freeIpaConfig",
		"genericOIDCConfig",
		"githubAppConfig",
		"githubConfig",
		"googleOauthConfig",
		"oidcConfig",
		"openLdapConfig",
	}
	for _, configType := range testAndApplyConfigs {
		c, ok := configMap[configType]
		s.Require().True(ok, "auth config %q not found", configType)
		_, hasAction := c.Actions["testAndApply"]
		s.Require().True(hasAction, "%s should have testAndApply action", configType)
	}

	// Configs that should have configureTest action.
	configureTestConfigs := []string{
		"azureADConfig",
		"cognitoConfig",
		"genericOIDCConfig",
		"githubAppConfig",
		"githubConfig",
		"googleOauthConfig",
		"oidcConfig",
	}
	for _, configType := range configureTestConfigs {
		c, ok := configMap[configType]
		s.Require().True(ok, "auth config %q not found", configType)
		_, hasAction := c.Actions["configureTest"]
		s.Require().True(hasAction, "%s should have configureTest action", configType)
	}

	// Configs that should have testAndEnable action.
	testAndEnableConfigs := []string{
		"adfsConfig",
		"genericSAMLConfig",
		"keyCloakConfig",
		"oktaConfig",
		"pingConfig",
		"shibbolethConfig",
	}
	for _, configType := range testAndEnableConfigs {
		c, ok := configMap[configType]
		s.Require().True(ok, "auth config %q not found", configType)
		_, hasAction := c.Actions["testAndEnable"]
		s.Require().True(hasAction, "%s should have testAndEnable action", configType)
	}
}

// TestAuthConfigSecrets verifies that updating a SAML auth config's spKey
// causes the corresponding secret to be created in the cattle-global-data
// namespace, and that secrets for other unconfigured SAML providers are not
// created.
func (s *AuthConfigTestSuite) TestAuthConfigSecrets() {
	s.T().Skip("Unable to create AuthConfigs with the client")
	pingConfig := createAuthConfig(s.T(), s.client.Management.AuthConfig, saml.PingName, client.PingConfigType)

	// Enable the config and set the spKey — the controller should create a
	// secret named "pingconfig-spkey" in the cattle-global-data namespace.
	_, err := s.client.Management.AuthConfig.Update(pingConfig, map[string]any{
		"spKey":   "-----BEGIN PRIVATE KEY-----",
		"enabled": true,
	})
	s.Require().NoError(err)

	s.T().Cleanup(func() {
		// Disable the config after the test.
		current, err := s.client.Management.AuthConfig.ByID("ping")
		if err == nil {
			_, _ = s.client.Management.AuthConfig.Update(current, map[string]any{
				"enabled": false,
			})
		}
	})

	dynamicClient, err := s.client.GetDownStreamClusterClient("local")
	s.Require().NoError(err)

	secretGVR := corev1.SchemeGroupVersion.WithResource("secrets")

	// Wait for the pingconfig-spkey secret to be created.
	s.Require().Eventually(func() bool {
		_, err := dynamicClient.Resource(secretGVR).Namespace("cattle-global-data").Get(
			context.TODO(), "pingconfig-spkey", metav1.GetOptions{})
		return err == nil
	}, 1*time.Minute, 2*time.Second, "timed out waiting for pingconfig-spkey secret")

	// Verify that secrets for other unconfigured SAML providers are NOT created.
	notExpected := []string{"adfsconfig-spkey", "oktaconfig-spkey", "keycloakconfig-spkey"}
	for _, name := range notExpected {
		_, err := dynamicClient.Resource(secretGVR).Namespace("cattle-global-data").Get(
			context.TODO(), name, metav1.GetOptions{})
		s.Require().Error(err, "secret %s should not exist", name)
	}
}

func TestAuthConfig(t *testing.T) {
	suite.Run(t, new(AuthConfigTestSuite))
}

func createAuthConfig(t *testing.T, authConfigs management.AuthConfigOperations, name, configType string) *management.AuthConfig {
	created, err := authConfigs.Create(
		&management.AuthConfig{
			Resource: types.Resource{
				ID: name,
			},
			Type:               configType,
			Enabled:            true,
			LogoutAllSupported: true,
		})
	require.NoError(t, err)

	return created
}

var authProviderTypes = map[string]string{
	activedirectory.ProviderName: client.ActiveDirectoryConfigType,
	azure.ProviderName:           client.AzureADConfigType,
	github.ProviderName:          client.GithubConfigType,
	githubapp.ProviderName:       client.GithubAppConfigType,
	local.Name:                   client.LocalConfigType,
	ldap.OpenLdapName:            client.OpenLdapConfigType,
	ldap.FreeIpaName:             client.FreeIpaConfigType,
	saml.PingName:                client.PingConfigType,
	saml.ADFSName:                client.ADFSConfigType,
	saml.KeyCloakName:            client.KeyCloakConfigType,
	saml.OKTAName:                client.OKTAConfigType,
	saml.ShibbolethName:          client.ShibbolethConfigType,
	saml.GenericSAMLName:         client.GenericSAMLConfigType,
	googleoauth.ProviderName:     client.GoogleOauthConfigType,
	oidc.ProviderName:            client.OIDCConfigType,
	keycloakoidc.ProviderName:    client.KeyCloakOIDCConfigType,
	genericoidc.ProviderName:     client.GenericOIDCConfigType,
	cognito.ProviderName:         client.CognitoConfigType,
}
