package feature_test

import (
	"context"
	"fmt"
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
	"testing"
	"time"
)

type AuthConfigSuite struct {
	suite.Suite
	ctx               context.Context
	cancel            context.CancelFunc
	testEnv           *envtest.Environment
	managementContext *config.ManagementContext
	wranglerContext   *wrangler.Context
}

func TestAuthConfigSuite(t *testing.T) {
	suite.Run(t, new(AuthConfigSuite))
}

func (s *AuthConfigSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Load CRD from YAML for REST Client
	s.testEnv = &envtest.Environment{}
	restCfg, err := s.testEnv.Start()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), restCfg)

	// Create CRDs
	factory, err := crd.NewFactoryFromClient(restCfg)
	assert.NoError(s.T(), err)

	err = factory.BatchCreateCRDs(s.ctx,
		crd.CRD{
			SchemaObject: v3.Token{},
			NonNamespace: true,
		},
		crd.CRD{
			SchemaObject: v3.AuthConfig{},
			NonNamespace: true,
		},
	).BatchWait()
	assert.NoError(s.T(), err)

	// Create the wrangler and management contexts
	wranglerContext, err := wrangler.NewContext(s.ctx, nil, restCfg)
	assert.NoError(s.T(), err)

	scaledContext, clusterManager, _, err := multiclustermanager.BuildScaledContext(s.ctx, wranglerContext, &multiclustermanager.Options{})
	assert.NoError(s.T(), err)
	s.managementContext, err = scaledContext.NewManagementContext()
	assert.NoError(s.T(), err)

	// Register the auth controller
	auth.RegisterEarly(s.ctx, s.managementContext, clusterManager)

	// Start controllers
	common.StartNormanControllers(s.ctx, s.T(), s.managementContext,
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "AuthConfig",
		},
	)

	// Start caches
	common.StartWranglerCaches(s.ctx, s.T(), s.managementContext.Wrangler,
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "Token",
		},
	)
}

func (s *AuthConfigSuite) TestTokensCleanup() {
	t := assert.New(s.T())

	const (
		tick         = 1 * time.Second
		duration     = 10 * time.Second
		numOfTokens  = 1000
		authProvider = "openldap"
	)

	// create the AuthConfig for the authProvider
	_, err := s.managementContext.Management.AuthConfigs("").Create(&v3.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: authProvider,
		},
		Type:    client.OpenLdapConfigType,
		Enabled: true,
	})
	t.NoError(err)

	// create some tokens for the authProvider
	for i := 0; i < numOfTokens; i++ {
		_, err := s.managementContext.Management.Tokens("").Create(&v3.Token{
			ObjectMeta:   metav1.ObjectMeta{GenerateName: "t-"},
			AuthProvider: authProvider,
		})
		t.NoError(err)
	}

	// and also some local tokens
	for i := 0; i < numOfTokens; i++ {
		_, err := s.managementContext.Management.Tokens("").Create(&v3.Token{
			ObjectMeta:   metav1.ObjectMeta{GenerateName: "t-"},
			AuthProvider: "local",
		})
		t.NoError(err)
	}

	fmt.Printf("created %d tokens\n", numOfTokens)

	// disable the provider, setting the cleanup annotation (eventually because of retries)
	authConfig, err := s.managementContext.Management.AuthConfigs("").Get(authProvider, metav1.GetOptions{})
	t.NoError(err)

	authConfig.Enabled = false
	authConfig.Annotations = map[string]string{auth.CleanupAnnotation: auth.CleanupUnlocked}
	authConfig, err = s.managementContext.Management.AuthConfigs("").Update(authConfig)
	t.NoError(err)

	// check that all the authProvider tokens are deleted
	ok := assert.EventuallyWithT(s.T(), func(c *assert.CollectT) {
		tokens, err := s.managementContext.Management.Tokens("").List(metav1.ListOptions{})
		assert.NoError(c, err)
		assert.NotNil(c, tokens)

		var remainingTokens int
		for _, token := range tokens.Items {
			if token.AuthProvider == authProvider {
				remainingTokens++
			}
		}

		fmt.Printf("remaining %d tokens\n", remainingTokens)
		assert.Zero(c, remainingTokens)
	}, duration, tick)

	t.True(ok, "secrets were left over after disabling auth configuration")
}

func (s *AuthConfigSuite) TearDownSuite() {
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err, "error shutting down environment")
}
