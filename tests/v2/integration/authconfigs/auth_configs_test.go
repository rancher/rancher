package integration

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/pkg/clientbase"
	"github.com/rancher/shepherd/pkg/session"
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

// TestAuthConfigsExistAndCannotBeDeleted verifies that the expected set of auth
// config types are returned by the API, and that attempting to delete any of
// them returns 405 Method Not Allowed.
func (s *AuthConfigTestSuite) TestAuthConfigsExistAndCannotBeDeleted() {
	configs, err := s.client.Management.AuthConfig.List(nil)
	s.Require().NoError(err)

	expectedTypes := map[string]bool{
		"activeDirectoryConfig": false,
		"adfsConfig":            false,
		"azureADConfig":         false,
		"cognitoConfig":         false,
		"freeIpaConfig":         false,
		"genericOIDCConfig":     false,
		"githubAppConfig":       false,
		"githubConfig":          false,
		"googleOauthConfig":     false,
		"keyCloakConfig":        false,
		"keyCloakOIDCConfig":    false,
		"localConfig":           false,
		"oidcConfig":            false,
		"oktaConfig":            false,
		"openLdapConfig":        false,
		"pingConfig":            false,
		"shibbolethConfig":      false,
	}

	for _, config := range configs.Data {
		if _, ok := expectedTypes[config.Type]; ok {
			expectedTypes[config.Type] = true
		} else {
			s.Failf("unexpected auth config type %q found in API response", config.Type)
		}
	}

	// Assert every expected auth config type was found.
	for configType, found := range expectedTypes {
		s.Require().True(found, "expected auth config type %q not found in API response", configType)
	}

	// Verify that deleting any auth config returns 405.
	for _, config := range configs.Data {
		c := config
		err := s.client.Management.AuthConfig.Delete(&c)
		s.Require().Error(err, "expected error deleting auth config %s", c.Type)

		var apiErr *clientbase.APIError
		s.Require().True(errors.As(err, &apiErr), "expected APIError for %s, got: %v", c.Type, err)
		s.Require().Equal(http.StatusMethodNotAllowed, apiErr.StatusCode, "expected 405 for %s", c.Type)
	}
}

// TestAuthConfigActions verifies that each auth config type exposes the
// expected set of actions (testAndApply, configureTest, testAndEnable).
func (s *AuthConfigTestSuite) TestAuthConfigActions() {
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
	pingConfig, err := s.client.Management.AuthConfig.ByID("ping")
	s.Require().NoError(err)

	// Enable the config and set the spKey — the controller should create a
	// secret named "pingconfig-spkey" in the cattle-global-data namespace.
	_, err = s.client.Management.AuthConfig.Update(pingConfig, map[string]any{
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
