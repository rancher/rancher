package oidc_integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	goidc "github.com/coreos/go-oidc/v3/oidc"
	gmux "github.com/gorilla/mux"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	providermocks "github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/controllers/management/oidcprovider"
	oprovider "github.com/rancher/rancher/pkg/oidc/provider"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/controllers/common"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	clientSecretsNamespace = "cattle-oidc-client-secrets"
	duration               = 10 * time.Second
	tick                   = 1 * time.Second
	state                  = "12345678910"
	fakeUser               = "fake-user"
	fakeToken              = "fake-token-name"
	fakeTokenValue         = "fake-token-value"
	fakeAuthProvider       = "auth-provider"
	fakeOIDCClient         = "oidc-client"
	fakeCodeVerifier       = "fake-code-verifiew"
)

type OIDCProviderSuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	testEnv         *envtest.Environment
	wranglerContext *wrangler.Context
	server          *httptest.Server
	wg              sync.WaitGroup
	done            chan struct{}
}

var (
	clientID     string
	clientSecret string
	provider     *goidc.Provider
	oauth2Config oauth2.Config
)

func (s *OIDCProviderSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.done = make(chan struct{})

	// Start envtest
	s.testEnv = &envtest.Environment{}
	restCfg, err := s.testEnv.Start()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), restCfg)

	// Register CRDs
	common.RegisterCRDs(s.ctx, s.T(), restCfg,
		crd.CRD{
			SchemaObject: apimgmtv3.Token{},
			NonNamespace: true,
		},
		crd.CRD{
			SchemaObject: apimgmtv3.User{},
			NonNamespace: true,
		},
		crd.CRD{
			SchemaObject: apimgmtv3.OIDCClient{},
			NonNamespace: true,
			Status:       true,
		},
	)

	// Create wrangler context
	s.wranglerContext, err = wrangler.NewContext(s.ctx, nil, restCfg)
	assert.NoError(s.T(), err)

	//register OIDCClient controller
	oidcprovider.Register(s.ctx, s.wranglerContext)

	// Init caches
	assert.NoError(s.T(), err)
	_, err = s.wranglerContext.ControllerFactory.SharedCacheFactory().ForKind(schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Version: "v3",
		Kind:    "OIDCClient",
	})
	_, err = s.wranglerContext.ControllerFactory.SharedCacheFactory().ForKind(schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Version: "v3",
		Kind:    "Token",
	})
	assert.NoError(s.T(), err)
	_, err = s.wranglerContext.ControllerFactory.SharedCacheFactory().ForKind(schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Version: "v3",
		Kind:    "User",
	})
	assert.NoError(s.T(), err)

	_, err = s.wranglerContext.ControllerFactory.SharedCacheFactory().ForKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Secret",
	})
	assert.NoError(s.T(), err)

	// Start caches
	common.StartWranglerCaches(s.ctx, s.T(), s.wranglerContext,
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "OIDCClient",
		},
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "Token",
		},
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "User",
		},
		schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Secret",
		})

	// Create and start the OIDCCLient controller factory
	cf := s.wranglerContext.ControllerFactory.ForResourceKind(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "oidcclients",
	}, "OIDCClient", false)
	assert.NoError(s.T(), err)
	err = cf.Start(s.ctx, 1)
	assert.NoError(s.T(), err)

	s.wranglerContext.ControllerFactory.SharedCacheFactory().WaitForCacheSync(s.ctx)

	_, err = s.wranglerContext.Core.Namespace().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cattle-system",
		},
	})
	assert.NoError(s.T(), err)

	// init OIDC provider
	mux := gmux.NewRouter()
	mux.UseEncodedPath()
	p, err := oprovider.NewProvider(s.ctx, s.wranglerContext.Mgmt.Token().Cache(), s.wranglerContext.Mgmt.Token(), s.wranglerContext.Mgmt.User().Cache(), s.wranglerContext.Mgmt.UserAttribute().Cache(), s.wranglerContext.Core.Secret().Cache(), s.wranglerContext.Core.Secret(), s.wranglerContext.Mgmt.OIDCClient().Cache(), s.wranglerContext.Mgmt.OIDCClient(), s.wranglerContext.Core.Namespace())
	assert.NoError(s.T(), err)
	p.RegisterOIDCProviderHandles(mux)
	// register redirect endpoint. This endpoint will be called by the OIDC provider with a valid code.
	mux.HandleFunc("/redirect", s.redirect)
	s.server = httptest.NewServer(mux)
}

func (s *OIDCProviderSuite) TestOIDCAuthorizationCodeFlow() {
	ctrl := gomock.NewController(s.T())

	// create user and token
	_, err := s.wranglerContext.Mgmt.User().Create(&apimgmtv3.User{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeUser,
		},
	})
	assert.NoError(s.T(), err)
	_, err = s.wranglerContext.Mgmt.Token().Create(&apimgmtv3.Token{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeToken,
			Labels: map[string]string{
				tokens.UserIDLabel: fakeUser,
			},
		},
		AuthProvider: fakeAuthProvider,
		Token:        fakeTokenValue,
		UserID:       fakeUser,
		Enabled:      ptr.To(true),
	})
	assert.NoError(s.T(), err)

	// mock auth provider
	mockProvider := providermocks.NewMockAuthProvider(ctrl)
	mockProvider.EXPECT().IsDisabledProvider().Return(false, nil).AnyTimes()
	providers.Providers[fakeAuthProvider] = mockProvider

	// create OIDC client
	oidcClient := &v3.OIDCClient{
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeOIDCClient,
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
		Spec: v3.OIDCClientSpec{
			RedirectURIs:                  []string{s.server.URL + "/redirect"},
			TokenExpirationSeconds:        3600,
			RefreshTokenExpirationSeconds: 36000,
		},
	}
	_, err = s.wranglerContext.Mgmt.OIDCClient().Create(oidcClient)
	assert.NoError(s.T(), err)

	// wait for clientID and clientSecret to be created by the controller
	require.EventuallyWithT(s.T(), func(c *assert.CollectT) {
		oidcClient, err = s.wranglerContext.Mgmt.OIDCClient().Get(oidcClient.Name, metav1.GetOptions{})
		assert.NoError(c, err)
		clientID = oidcClient.Status.ClientID
		assert.NotEmpty(c, clientID)
		secret, err := s.wranglerContext.Core.Secret().Get(clientSecretsNamespace, clientID, metav1.GetOptions{})
		assert.NoError(c, err)
		clientSecret = string(secret.Data["client-secret-1"])
		assert.NotEmpty(c, clientSecret)
	}, duration, tick)

	// set server URL
	err = settings.ServerURL.Set(s.server.URL)
	assert.NoError(s.T(), err)

	// configure go oidc client
	provider, err = goidc.NewProvider(s.ctx, s.server.URL+"/oidc")
	assert.NoError(s.T(), err)

	oauth2Config = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  s.server.URL + "/redirect",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{goidc.ScopeOpenID, "profile", "offline_access"},
	}

	authURL := oauth2Config.AuthCodeURL(state, oauth2.S256ChallengeOption(fakeCodeVerifier))
	req, err := http.NewRequest(http.MethodGet, authURL, nil)
	require.NoError(s.T(), err)
	req.Header.Set("Authorization", "Bearer "+fakeToken+":"+fakeTokenValue)
	s.wg.Add(1)
	res, err := http.DefaultClient.Do(req)
	require.NoError(s.T(), err)
	defer res.Body.Close()
	assert.Equal(s.T(), http.StatusOK, res.StatusCode)

	// use a waitGroup to ensure the asserts inside the redirect function were executed.
	go func() {
		s.wg.Wait()
		close(s.done)
	}()
	select {
	case <-time.After(2 * time.Second):
		assert.Fail(s.T(), "timeout waiting for redirect to finish")
	case <-s.done:
	}
}

// redirect will be called with a valid code as part of the OIDC authorization code flow.
// The code is exchanged for id_token, access_token, and refresh_token.
func (s *OIDCProviderSuite) redirect(rw http.ResponseWriter, r *http.Request) {
	defer s.wg.Done()
	// errors are returned in the error query parameter as per the OIDC spec.
	oidcErr := r.URL.Query().Get("error")
	if oidcErr != "" {
		assert.Fail(s.T(), oidcErr)
		return
	}

	oauth2Token, err := oauth2Config.Exchange(s.ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(fakeCodeVerifier))
	if err != nil {
		http.Error(rw, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		assert.Fail(s.T(), err.Error())
		return
	}
	s.verifyTokens(oauth2Token.Extra("id_token").(string), oauth2Token.AccessToken, oauth2Token.RefreshToken)

	// Obtain new tokens using the refresh_token
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", oauth2Token.RefreshToken)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	req, err := http.NewRequest(http.MethodPost, provider.Endpoint().TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		http.Error(rw, "Failed to create HTTP request: "+err.Error(), http.StatusInternalServerError)
		assert.Fail(s.T(), err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(rw, "Failed to send HTTP request: "+err.Error(), http.StatusInternalServerError)
		assert.Fail(s.T(), err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		assert.Fail(s.T(), "Failed to refresh token: %s\nResponse: %s\n", resp.Status, string(body))
		http.Error(rw, "Failed to refresh token", http.StatusInternalServerError)
		return
	}

	var refreshTokenResponse *oprovider.TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&refreshTokenResponse)
	assert.NoError(s.T(), err)

	s.verifyTokens(refreshTokenResponse.IDToken, refreshTokenResponse.AccessToken, refreshTokenResponse.RefreshToken)
}

func (s *OIDCProviderSuite) verifyTokens(idTokenStr string, accessTokenStr string, refreshTokenStr string) {
	// verify id token
	verifier := provider.Verifier(&goidc.Config{ClientID: clientID})
	idToken, err := verifier.Verify(s.ctx, idTokenStr)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), []string{clientID}, idToken.Audience)
	assert.Equal(s.T(), fakeUser, idToken.Subject)
	assert.Equal(s.T(), s.server.URL+"/oidc", idToken.Issuer)

	//verify access token
	accessToken, err := verifier.Verify(s.ctx, accessTokenStr)
	assert.NoError(s.T(), err)
	claims := map[string]interface{}{}
	err = accessToken.Claims(&claims)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), []string{clientID}, accessToken.Audience)
	assert.Equal(s.T(), fakeUser, accessToken.Subject)
	assert.Equal(s.T(), s.server.URL+"/oidc", accessToken.Issuer)
	assert.ElementsMatch(s.T(), []string{"openid", "profile", "offline_access"}, claims["scope"])

	// verify refresh token
	refreshVerifier := provider.Verifier(&goidc.Config{
		ClientID:        clientID,
		SkipIssuerCheck: true,
	})
	refreshToken, err := refreshVerifier.Verify(s.ctx, refreshTokenStr)
	assert.NoError(s.T(), err)
	hash := sha256.Sum256([]byte(fakeToken))
	rancherTokenHash := hex.EncodeToString(hash[:])
	claims = map[string]interface{}{}
	err = refreshToken.Claims(&claims)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), []string{clientID}, refreshToken.Audience)
	assert.Equal(s.T(), fakeUser, refreshToken.Subject)
	assert.Equal(s.T(), rancherTokenHash, claims["rancher_token_hash"])
	assert.ElementsMatch(s.T(), []string{"openid", "profile", "offline_access"}, claims["scope"])
}

func TestOIDCProviderSuite(t *testing.T) {
	suite.Run(t, new(OIDCProviderSuite))
}

func (s *OIDCProviderSuite) TearDownSuite() {
	s.server.Close()
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err)
}
