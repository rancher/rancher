package authconfig_test

import (
	"context"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/controllers/common"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type AuthConfigSuite struct {
	suite.Suite
	ctx               context.Context
	cancel            context.CancelFunc
	testEnv           *envtest.Environment
	managementContext *config.ManagementContext
}

func TestAuthConfigSuite(t *testing.T) {
	suite.Run(t, new(AuthConfigSuite))
}

func (s *AuthConfigSuite) SetupSuite() {
	t := s.T()
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Start envtest.
	s.testEnv = &envtest.Environment{}
	restCfg, err := s.testEnv.Start()
	assert.NoError(t, err)
	assert.NotNil(t, restCfg)

	// Create CRDs.
	factory, err := crd.NewFactoryFromClient(restCfg)
	assert.NoError(t, err)

	err = factory.BatchCreateCRDs(s.ctx,
		crd.CRD{
			SchemaObject: v3.Token{},
			NonNamespace: true,
		},
		crd.CRD{
			SchemaObject: v3.AuthConfig{},
			NonNamespace: true,
		},
		crd.CRD{
			SchemaObject: v3.User{},
			NonNamespace: true,
		},
	).BatchWait()
	assert.NoError(t, err)

	// Create the wrangler and management contexts
	wranglerContext, err := wrangler.NewContext(s.ctx, nil, restCfg)
	assert.NoError(t, err)

	scaledContext, clusterManager, _, err := multiclustermanager.BuildScaledContext(s.ctx, wranglerContext, &multiclustermanager.Options{})
	assert.NoError(t, err)
	s.managementContext, err = scaledContext.NewManagementContext()
	assert.NoError(t, err)

	// Register the auth controller
	auth.RegisterEarly(s.ctx, s.managementContext, clusterManager)

	// Start controllers
	common.StartNormanControllers(s.ctx, t, s.managementContext,
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "AuthConfig",
		},
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "User",
		},
	)

	// Start caches
	common.StartWranglerCaches(s.ctx, t, s.managementContext.Wrangler,
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "Token",
		},
	)
}

func (s *AuthConfigSuite) TestTokensCleanup() {
	const (
		tick         = 1 * time.Second
		duration     = 10 * time.Second
		numOfTokens  = 2
		authProvider = "openldap"
	)

	t := s.T()

	authConfigClient := s.managementContext.Management.AuthConfigs("")
	tokenClient := s.managementContext.Management.Tokens("")

	// create the AuthConfig for the authProvider
	_, err := authConfigClient.Create(&v3.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: authProvider,
			Annotations: map[string]string{
				auth.CleanupAnnotation: auth.CleanupUnlocked,
			},
		},
		Type:    client.OpenLdapConfigType,
		Enabled: true,
	})
	assert.NoError(t, err)

	// create some tokens for the authProvider
	for i := 0; i < numOfTokens; i++ {
		_, err := tokenClient.Create(&v3.Token{
			ObjectMeta:   metav1.ObjectMeta{GenerateName: "t-"},
			AuthProvider: authProvider,
			UserPrincipal: v3.Principal{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openldap_user://uid=user,dc=example,dc=org",
				},
				PrincipalType: "user",
				Provider:      "openldap",
			},
		})
		assert.NoError(t, err)
	}

	// and also some local tokens
	for i := 0; i < numOfTokens; i++ {
		_, err := tokenClient.Create(&v3.Token{
			ObjectMeta:   metav1.ObjectMeta{GenerateName: "t-"},
			AuthProvider: "local",
			UserPrincipal: v3.Principal{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local://u-abcdef",
				},
				PrincipalType: "user",
				Provider:      "local",
			},
		})
		assert.NoError(t, err)
	}

	// disable the provider, setting the cleanup annotation (eventually because of retries)
	authConfig, err := authConfigClient.Get(authProvider, metav1.GetOptions{})
	assert.NoError(t, err)

	authConfig.Enabled = false
	authConfig.Annotations = map[string]string{auth.CleanupAnnotation: auth.CleanupUnlocked}
	_, err = authConfigClient.Update(authConfig)
	assert.NoError(t, err)

	// check that all the authProvider tokens are deleted
	ok := assert.EventuallyWithT(t, func(c *assert.CollectT) {
		tokens, err := tokenClient.List(metav1.ListOptions{})
		assert.NoError(c, err)
		assert.NotNil(c, tokens)

		var remainingTokens int
		for _, token := range tokens.Items {
			if token.AuthProvider == authProvider {
				remainingTokens++
			}
		}

		t.Logf("remaining %d tokens\n", remainingTokens)
		assert.Zero(c, remainingTokens)
	}, duration, tick)

	assert.True(t, ok, "secrets were left over after disabling auth configuration")
}

func (s *AuthConfigSuite) TearDownSuite() {
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err, "error shutting down environment")
}
