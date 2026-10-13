package saml

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/types"
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

func TestConfiguredProviderContainsLdapProvider(t *testing.T) {
	for _, providerName := range []string{
		"okta",
		"adfs",
	} {
		t.Run(providerName+" has ldap configuration", func(t *testing.T) {
			// Attention: ADFS/LDAP search is a prime feature
			if providerName == "adfs" {
				features.ADFSLDAPSearch.Set(true)
				t.Cleanup(features.ADFSLDAPSearch.Unset)
				t.Setenv("RANCHER_VERSION_TYPE", "prime")
			}

			// saml.Configure runs some ldap specific logic based on the saml provider name, so we provide
			// just enough scaffolding to run the Configure function.
			ctx := t.Context()
			mgmtCtx, err := config.NewScaledContext(rest.Config{}, nil)
			require.NoError(t, err, "Failed to create NewScaledContext")
			mgmtCtx.RunContext = ctx

			// Create the dummy wrangler context
			wranglerContext, err := wrangler.NewContext(ctx, nil, &rest.Config{})
			require.NoError(t, err, "Failed to create wranglerContext")
			mgmtCtx.Wrangler = wranglerContext

			tokenMGR := tokens.NewManager(wranglerContext)
			provider, ok := Configure(t.Context(), mgmtCtx, mgmtCtx.UserManager, tokenMGR, providerName).(*Provider)
			require.True(t, ok, "Failed to Configure a valid Provider")

			assert.True(t, provider.hasLdapGroupSearch(), providerName+": Missing LDAP group search capability for provider")
			assert.NotNil(t, provider.ldapProvider, providerName+": Configured provider did not receive child LDAP provider")
		})
	}
}

func TestConfiguredGenericSAMLProviderHasNoLdap(t *testing.T) {
	// saml.Configure runs some ldap specific logic based on the saml provider name, so we provide
	// just enough scaffolding to run the Configure function.
	ctx := t.Context()
	mgmtCtx, err := config.NewScaledContext(rest.Config{}, nil)
	require.NoError(t, err, "Failed to create NewScaledContext")

	// Create the dummy wrangler context
	wranglerContext, err := wrangler.NewContext(ctx, nil, &rest.Config{})
	require.NoError(t, err, "Failed to create wranglerContext")
	mgmtCtx.Wrangler = wranglerContext

	tokenMGR := tokens.NewManager(wranglerContext)
	provider, ok := Configure(ctx, mgmtCtx, mgmtCtx.UserManager, tokenMGR, GenericSAMLName).(*Provider)
	require.True(t, ok, "Failed to Configure a valid Provider")

	assert.False(t, provider.hasLdapGroupSearch(), "Generic SAML provider must not have LDAP group search")
	assert.Nil(t, provider.ldapProvider, "Generic SAML provider must not receive a child LDAP provider")
}

func TestNonPrimeADFSProviderHasNoLdap(t *testing.T) {
	// ADFS / LDAP is prime gated. Here we verify that LDAP is not present for a non-prime setup
	t.Setenv("RANCHER_VERSION_TYPE", "")

	// saml.Configure runs some ldap specific logic based on the saml provider name, so we provide
	// just enough scaffolding to run the Configure function.
	ctx := t.Context()
	mgmtCtx, err := config.NewScaledContext(rest.Config{}, nil)
	require.NoError(t, err, "Failed to create NewScaledContext")

	// Create the dummy wrangler context
	wranglerContext, err := wrangler.NewContext(ctx, nil, &rest.Config{})
	require.NoError(t, err, "Failed to create wranglerContext")
	mgmtCtx.Wrangler = wranglerContext

	tokenMGR := tokens.NewManager(wranglerContext)

	provider, ok := Configure(ctx, mgmtCtx, mgmtCtx.UserManager, tokenMGR, ADFSName).(*Provider)
	require.True(t, ok, "Failed to Configure a valid Provider")

	assert.False(t, provider.hasLdapGroupSearch(), "ADFS provider must not have LDAP group search for non-prime")
	assert.Nil(t, provider.ldapProvider, "ADFS SAML provider must not receive a child LDAP provider for non-prime")
}

func TestCombineSamlAndLdapConfigStoresLDAPPasswordInSecret(t *testing.T) {
	originalGetLDAPConfig := getLDAPConfig
	t.Cleanup(func() {
		getLDAPConfig = originalGetLDAPConfig
	})

	var gotLDAPConfigName string
	getLDAPConfig = func(_ common.AuthProvider, configName string) (*apiv3.LdapConfig, *x509.CertPool, error) {
		gotLDAPConfigName = configName
		return &apiv3.LdapConfig{
			LdapFields: apiv3.LdapFields{
				ServiceAccountPassword: "test-password",
			},
		}, nil, nil
	}

	tests := []struct {
		name           string
		providerName   string
		configName     string
		configType     string
		wantSecretName string
	}{
		{
			name:           "okta",
			providerName:   OKTAName,
			configType:     client.OKTAConfigType,
			wantSecretName: "oktaconfig-serviceaccountpassword",
		},
		{
			name:           "default okta config",
			providerName:   OKTAName,
			configName:     "okta",
			configType:     client.OKTAConfigType,
			wantSecretName: "oktaconfig-serviceaccountpassword",
		},
		{
			name:           "additional okta config",
			providerName:   OKTAName,
			configName:     "okta-eu",
			configType:     client.OKTAConfigType,
			wantSecretName: "oktaconfig-okta-eu-serviceaccountpassword",
		},
		{
			name:           "shibboleth",
			providerName:   ShibbolethName,
			configType:     client.ShibbolethConfigType,
			wantSecretName: "shibbolethconfig-serviceaccountpassword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			secretController := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secretCache := wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
			secretController.EXPECT().Cache().Return(secretCache)
			secretCache.EXPECT().Get(common.SecretsNamespace, tt.wantSecretName).Return(nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, tt.wantSecretName))
			secretController.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
				assert.Equal(t, tt.wantSecretName, secret.Name)
				assert.Equal(t, common.SecretsNamespace, secret.Namespace)
				assert.Equal(t, map[string]string{
					"serviceaccountpassword": "test-password",
				}, secret.StringData)

				return secret, nil
			})

			provider := &Provider{
				name:         tt.providerName,
				secrets:      secretController,
				ldapProvider: &mockLdapProvider{providerName: tt.providerName},
			}

			config, err := provider.combineSamlAndLdapConfig(&apiv3.SamlConfig{
				AuthConfig: apiv3.AuthConfig{
					ObjectMeta: metav1.ObjectMeta{Name: tt.configName},
					Type:       tt.configType,
				},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.configName, gotLDAPConfigName, "LDAP config must be read from the SAML config being saved")

			wantSecretRef := common.SecretsNamespace + ":" + tt.wantSecretName

			switch typedConfig := config.(type) {
			case *apiv3.OKTAConfig:
				assert.Equal(t, wantSecretRef, typedConfig.OpenLdapConfig.ServiceAccountPassword)
				assert.True(t, apiv3.AuthConfigOKTAPasswordMigrated.IsTrue(&typedConfig.SamlConfig))
			case *apiv3.ShibbolethConfig:
				assert.Equal(t, wantSecretRef, typedConfig.OpenLdapConfig.ServiceAccountPassword)
				assert.True(t, apiv3.AuthConfigConditionSecretsMigrated.IsTrue(&typedConfig.SamlConfig))
			default:
				t.Fatalf("unexpected config type %T", config)
			}
		})
	}
}

func TestSaveSamlConfigReturnsErrorWhenLDAPPasswordSecretSaveFails(t *testing.T) {
	originalGetLDAPConfig := getLDAPConfig
	t.Cleanup(func() {
		getLDAPConfig = originalGetLDAPConfig
	})

	getLDAPConfig = func(common.AuthProvider, string) (*apiv3.LdapConfig, *x509.CertPool, error) {
		return &apiv3.LdapConfig{
			LdapFields: apiv3.LdapFields{
				ServiceAccountPassword: "test-password",
			},
		}, nil, nil
	}

	ctrl := gomock.NewController(t)
	secretController := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secretCache := wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretController.EXPECT().Cache().Return(secretCache)
	secretCache.EXPECT().Get(common.SecretsNamespace, "oktaconfig-serviceaccountpassword").Return(nil, assert.AnError)

	provider := &Provider{
		name:         OKTAName,
		secrets:      secretController,
		ldapProvider: &mockLdapProvider{providerName: OKTAName},
		authConfigs: &fakes.AuthConfigInterfaceMock{
			ObjectClientFunc: func() *objectclient.ObjectClient {
				t.Fatal("auth config update should not be attempted when saving the LDAP password secret fails")
				return nil
			},
		},
		getSamlConfig: func(string) (*apiv3.SamlConfig, error) {
			return &apiv3.SamlConfig{
				AuthConfig: apiv3.AuthConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "okta"},
				},
			}, nil
		},
	}

	err := provider.saveSamlConfig(&apiv3.SamlConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to save ldap service account password")
}

func TestSaveSamlConfigUsesConfigNameForSpKeySecret(t *testing.T) {
	ctrl := gomock.NewController(t)
	secretController := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secretCache := wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretController.EXPECT().Cache().Return(secretCache)
	secretCache.EXPECT().Get(common.SecretsNamespace, "genericsamlconfig-genericsaml-eu-spkey").Return(nil, assert.AnError)

	provider := &Provider{
		name:    GenericSAMLName,
		secrets: secretController,
		getSamlConfig: func(string) (*apiv3.SamlConfig, error) {
			return &apiv3.SamlConfig{
				AuthConfig: apiv3.AuthConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "genericsaml-eu"},
				},
			}, nil
		},
	}

	err := provider.saveSamlConfig(&apiv3.SamlConfig{
		AuthConfig: apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "genericsaml-eu"}},
		SpKey:      "test-key",
	})
	require.ErrorIs(t, err, assert.AnError)
}

func TestSearchPrincipals(t *testing.T) {
	for _, providerName := range []string{
		"okta",
		"adfs",
	} {
		t.Run(providerName, func(t *testing.T) {
			// Attention: ADFS/LDAP search is a prime feature
			if providerName == "adfs" {
				features.ADFSLDAPSearch.Set(true)
				t.Cleanup(features.ADFSLDAPSearch.Unset)
				t.Setenv("RANCHER_VERSION_TYPE", "prime")
			}

			userType := providerName + "_user"
			groupType := providerName + "_group"

			tests := []struct {
				desc             string
				searchKey        string
				principalType    string
				isLdapConfigured bool
				principals       []string
			}{
				{
					desc:             "search for user with ldap",
					isLdapConfigured: true,
					searchKey:        "al",
					principalType:    common.UserPrincipalType,
					principals: []string{
						userType + "://alice",
					},
				},
				{
					desc:             "search for user without ldap",
					isLdapConfigured: false,
					searchKey:        "alice",
					principalType:    common.UserPrincipalType,
					principals: []string{
						userType + "://alice",
					},
				},
				{
					desc:             "search for group without ldap",
					isLdapConfigured: false,
					searchKey:        "admins",
					principalType:    common.GroupPrincipalType,
					principals: []string{
						groupType + "://admins",
					},
				},
				{
					desc:             "search for any principal without ldap",
					isLdapConfigured: false,
					searchKey:        "dev",
					principalType:    "",
					principals: []string{
						userType + "://dev",
						groupType + "://dev",
					},
				},
			}

			token := &apiv3.Token{
				AuthProvider: "okta",
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: userType + "://00ux3opnquJigzHYx697",
					},
					LoginName:     "developer",
					PrincipalType: "user",
				},
			}

			extToken := &ext.Token{
				Spec: ext.TokenSpec{
					UserPrincipal: ext.TokenPrincipal{
						Name:          userType + "://00ux3opnquJigzHYx697",
						LoginName:     "developer",
						PrincipalType: "user",
						Provider:      "otka",
					},
				},
			}

			for _, tt := range tests {
				t.Run(tt.desc, func(t *testing.T) {
					provider := &Provider{
						name: providerName,
						ldapProvider: &mockLdapProvider{
							providerName:     providerName,
							isLdapConfigured: tt.isLdapConfigured,
						},
					}

					results, err := provider.SearchPrincipals(tt.searchKey, tt.principalType, token)
					require.NoError(t, err)
					require.Len(t, results, len(tt.principals))
					for _, principal := range results {
						assert.Contains(t, tt.principals, principal.Name)
					}
				})

				// same behaviour for ext tokens
				t.Run(tt.desc+", ext", func(t *testing.T) {
					provider := &Provider{
						name: providerName,
						ldapProvider: &mockLdapProvider{
							providerName:     providerName,
							isLdapConfigured: tt.isLdapConfigured,
						},
					}

					results, err := provider.SearchPrincipals(tt.searchKey, tt.principalType, extToken)
					require.NoError(t, err)
					require.Len(t, results, len(tt.principals))
					for _, principal := range results {
						assert.Contains(t, tt.principals, principal.Name)
					}
				})
			}
		})
	}
}

func TestSearchPrincipalsNonPrime(t *testing.T) {
	// ADFS / LDAP search is prime gated. Here we verify the non-prime behaviour
	for _, providerName := range []string{
		"adfs",
	} {
		t.Run(providerName, func(t *testing.T) {
			userType := providerName + "_user"

			t.Setenv("RANCHER_VERSION_TYPE", "")

			tests := []struct {
				desc             string
				searchKey        string
				principalType    string
				isLdapConfigured bool
				principals       []string
			}{
				{
					desc:             "search for user with ldap is not",
					isLdapConfigured: true,
					searchKey:        "al",
					principalType:    common.UserPrincipalType,
					principals:       []string{userType + "://al"},
				},
			}

			token := &apiv3.Token{
				AuthProvider: "adfs",
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: "adfs_user://00ux3opnquJigzHYx697",
					},
					LoginName:     "developer",
					PrincipalType: "user",
				},
			}

			for _, tt := range tests {
				tt := tt
				t.Run(tt.desc, func(t *testing.T) {
					provider := &Provider{
						name: providerName,
						ldapProvider: &mockLdapProvider{
							providerName:     providerName,
							isLdapConfigured: tt.isLdapConfigured,
						},
					}

					results, err := provider.SearchPrincipals(tt.searchKey, tt.principalType, token)
					require.NoError(t, err)
					require.Len(t, results, len(tt.principals))
					for _, principal := range results {
						assert.Contains(t, tt.principals, principal.Name)
					}
				})

				// same behaviour for ext tokens
				t.Run(tt.desc+", ext", func(t *testing.T) {
					provider := &Provider{
						name: providerName,
						ldapProvider: &mockLdapProvider{
							providerName:     providerName,
							isLdapConfigured: tt.isLdapConfigured,
						},
					}

					results, err := provider.SearchPrincipals(tt.searchKey, tt.principalType, token)
					require.NoError(t, err)
					require.Len(t, results, len(tt.principals))
					for _, principal := range results {
						assert.Contains(t, tt.principals, principal.Name)
					}
				})
			}
		})
	}
}

func TestLogoutAllInvalidFinalRedirectURL(t *testing.T) {
	providerName := "test-provider"
	userName := "test-user"
	invalidRedirect := "https://attacker.example.com/logout"

	metadataURL, err := url.Parse("https://rancher.example.com/v1-saml/" + providerName + "/metadata")
	require.NoError(t, err)

	serviceProvider := &saml.ServiceProvider{
		MetadataURL: *metadataURL,
	}

	SamlProvidersOriginal := SamlProviders[providerName]
	provider := &Provider{
		serviceProvider: serviceProvider,
		name:            providerName,
		userMGR: &fakeUserManager{
			userName: userName,
			userAttribute: &apiv3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{
					providerName: {
						"username": {"idp-username"},
					},
				},
			},
		},
		clientState: &fakeClientState{},
		sloEnabled:  true,
	}
	SamlProviders[providerName] = provider
	t.Cleanup(func() {
		SamlProviders[providerName] = SamlProvidersOriginal
	})

	body := bytes.NewBufferString(`{"finalRedirectUrl":"` + invalidRedirect + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1-saml/"+providerName+"/logout", body)
	res := httptest.NewRecorder()
	token := &fakeToken{authProvider: providerName}

	err = provider.LogoutAll(res, req, token)
	assert.ErrorContains(t, err, "Invalid redirect URL 400: failed to logout")
}

func TestPerformSamlLoginSetsStateAndResponds(t *testing.T) {
	providerName := "test-provider"
	finalRedirect := "https://rancher.example.com/dashboard"
	publicKey := "rsa-public-key"
	requestID := "req-12345"
	responseType := "code"

	metadataURL := testParseURL(t, "https://rancher.example.com/v1-saml/"+providerName+"/metadata")
	acsURL := testParseURL(t, "https://rancher.example.com/v1-saml/"+providerName+"/acs")

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serviceProvider := &saml.ServiceProvider{
		Key:         privateKey,
		MetadataURL: metadataURL,
		AcsURL:      acsURL,
		IDPMetadata: &saml.EntityDescriptor{
			IDPSSODescriptors: []saml.IDPSSODescriptor{{
				SingleSignOnServices: []saml.Endpoint{{
					Binding:  saml.HTTPRedirectBinding,
					Location: "https://idp.example.com/sso",
				}},
			}},
		},
	}

	clientState := newRecordingClientState()
	provider := &Provider{
		serviceProvider: serviceProvider,
		clientState:     clientState,
	}

	originalProvider, exists := SamlProviders[providerName]
	SamlProviders[providerName] = provider
	t.Cleanup(func() {
		if exists {
			SamlProviders[providerName] = originalProvider
			return
		}
		delete(SamlProviders, providerName)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1-saml/"+providerName+"/login", nil)
	res := httptest.NewRecorder()
	loginInput := &apiv3.SamlLoginInput{
		FinalRedirectURL: finalRedirect,
		PublicKey:        publicKey,
		RequestID:        requestID,
		ResponseType:     responseType,
	}

	err = PerformSamlLogin(req, res, providerName, loginInput, provider)
	require.NoError(t, err)

	assert.Equal(t, serviceProvider.AcsURL.Path, clientState.path)
	assert.Equal(t, finalRedirect, clientState.states["Rancher_FinalRedirectURL"])
	assert.Equal(t, loginAction, clientState.states["Rancher_Action"])
	assert.Equal(t, publicKey, clientState.states["Rancher_PublicKey"])
	assert.Equal(t, requestID, clientState.states["Rancher_RequestID"])
	assert.Equal(t, responseType, clientState.states["Rancher_ResponseType"])

	assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
	var output struct {
		IDPRedirectURL string `json:"idpRedirectUrl"`
		Type           string `json:"type"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&output))
	assert.Equal(t, "samlLoginOutput", output.Type)
	assert.NotEmpty(t, output.IDPRedirectURL)
}

func TestPerformSamlLoginRejectsInvalidRedirect(t *testing.T) {
	providerName := "test-provider"

	metadataURL, err := url.Parse("https://rancher.example.com/v1-saml/" + providerName + "/metadata")
	require.NoError(t, err)

	provider := &Provider{
		serviceProvider: &saml.ServiceProvider{MetadataURL: *metadataURL},
		clientState:     newRecordingClientState(),
	}
	setupSamlProviderTypes(t)
	setSamlProvider(providerName, provider)

	req := httptest.NewRequest(http.MethodPost, "/v1-saml/"+providerName+"/login", nil)
	res := httptest.NewRecorder()
	loginInput := &apiv3.SamlLoginInput{FinalRedirectURL: "https://attacker.example.com/login"}

	err = PerformSamlLogin(req, res, providerName, loginInput, provider)
	assert.ErrorContains(t, err, "Invalid redirect URL 400: failed to login")
	assert.Empty(t, res.Body.String())
	assert.Empty(t, res.Header().Get("Content-Type"))
}

var _ ClientState = (*recordingClientState)(nil)

type recordingClientState struct {
	path   string
	states map[string]string
}

func newRecordingClientState() *recordingClientState {
	return &recordingClientState{states: map[string]string{}}
}

func (m *recordingClientState) SetPath(path string) {
	m.path = path
}

func (m *recordingClientState) SetState(w http.ResponseWriter, r *http.Request, id string, value string) {
	m.states[id] = value
}

func (m *recordingClientState) GetStates(r *http.Request) map[string]string {
	return m.states
}

func (m *recordingClientState) GetState(r *http.Request, id string) string {
	return m.states[id]
}

func (m *recordingClientState) DeleteState(w http.ResponseWriter, r *http.Request, id string) error {
	delete(m.states, id)
	return nil
}

var _ ClientState = (*fakeClientState)(nil)

type fakeClientState struct{}

func (m *fakeClientState) SetPath(path string) {}

func (m *fakeClientState) SetState(w http.ResponseWriter, r *http.Request, id string, value string) {}

func (m *fakeClientState) GetStates(r *http.Request) map[string]string { return nil }

func (m *fakeClientState) GetState(r *http.Request, id string) string { return "" }

func (m *fakeClientState) DeleteState(w http.ResponseWriter, r *http.Request, id string) error {
	return nil
}

var _ user.Manager = (*fakeUserManager)(nil)

type fakeUserManager struct {
	userName      string
	userAttribute *apiv3.UserAttribute
}

func (m *fakeUserManager) GetUser(r *http.Request) string { return m.userName }

func (m *fakeUserManager) EnsureUser(principalName, displayName string) (*apiv3.User, error) {
	return nil, nil
}

func (m *fakeUserManager) CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []apiv3.Principal) (bool, error) {
	return true, nil
}

func (m *fakeUserManager) SetPrincipalOnCurrentUserByUserID(userID string, principal apiv3.Principal) (*apiv3.User, error) {
	return nil, nil
}

func (m *fakeUserManager) SetPrincipalOnCurrentUser(r *http.Request, principal apiv3.Principal) (*apiv3.User, error) {
	return nil, nil
}

func (m *fakeUserManager) CreateNewUserClusterRoleBinding(userName string, userUID apitypes.UID) error {
	return nil
}

func (m *fakeUserManager) GetUserByPrincipalID(principalName string) (*apiv3.User, error) {
	return nil, nil
}

func (m *fakeUserManager) GetGroupsForTokenAuthProvider(token accessor.TokenAccessor) []apiv3.Principal {
	return nil
}

func (m *fakeUserManager) EnsureAndGetUserAttribute(userID string) (*apiv3.UserAttribute, bool, error) {
	return m.userAttribute, false, nil
}

func (m *fakeUserManager) IsMemberOf(token accessor.TokenAccessor, group apiv3.Principal) bool {
	return false
}

func (m *fakeUserManager) UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []apiv3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error {
	return nil
}

func (m *fakeUserManager) UserAttributeCreateOrUpdateNoGroups(userID, provider string, userExtraInfo map[string][]string, loginTime ...time.Time) error {
	return nil
}

var _ accessor.TokenAccessor = (*fakeToken)(nil)

type fakeToken struct {
	authProvider string
}

func (m *fakeToken) GetLabels() map[string]string { return nil }

func (m *fakeToken) GetFullName() string { return "" }

func (m *fakeToken) GetKind() string { return "fake" }

func (m *fakeToken) GetName() string { return "" }

func (m *fakeToken) GetIsEnabled() bool { return true }

func (m *fakeToken) GetIsDerived() bool { return false }

func (m *fakeToken) GetAuthProvider() string { return m.authProvider }

func (m *fakeToken) GetUserID() string { return "" }

func (m *fakeToken) GetProviderInfo() map[string]string { return nil }

func (m *fakeToken) ObjClusterName() string { return "" }

func (m *fakeToken) GetUserPrincipal() apiv3.Principal { return apiv3.Principal{} }

func (m *fakeToken) GetGroupPrincipals() []apiv3.Principal { return nil }

func (m *fakeToken) GetLastUsedAt() *metav1.Time { return nil }

func (m *fakeToken) GetLastActivitySeen() *metav1.Time { return nil }

func (m *fakeToken) GetCreationTime() metav1.Time { return metav1.Time{} }

func (m *fakeToken) GetExpiresAt() string { return "" }

func (m *fakeToken) GetIsExpired() bool { return false }

// Bare minimum to provide ldap responses (or error conditions) when performing SearchPrincipals. We're testing
// the SAML provider's logic, not anything the ldap provider is doing, so we merely need enough scaffolding to
// detect that the ldapProvider was used at all.
type mockLdapProvider struct {
	providerName     string
	isLdapConfigured bool
}

func (p *mockLdapProvider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockLdapProvider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockLdapProvider) GetName() string {
	return p.providerName
}

func (p *mockLdapProvider) AuthenticateUser(http.ResponseWriter, *http.Request, any) (apiv3.Principal, []apiv3.Principal, string, error) {
	panic("AuthenticateUser Unimplemented!")
}

func (p *mockLdapProvider) SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]apiv3.Principal, error) {
	if !p.isLdapConfigured {
		return nil, ldap.ErrorNotConfigured{}
	}

	return []apiv3.Principal{{
		ObjectMeta:    metav1.ObjectMeta{Name: p.providerName + "_" + principalType + "://alice"},
		DisplayName:   "Alice",
		LoginName:     "alice",
		PrincipalType: "user",
		Me:            true,
		Provider:      p.providerName,
	}}, nil
}

func (p *mockLdapProvider) CustomizeSchema(schema *types.Schema) {
	panic("CustomizeSchema Unimplemented!")
}

func (p *mockLdapProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	panic("GetPrincipal Unimplemented!")
}

func (p *mockLdapProvider) TransformToAuthProvider(authConfig map[string]any) (map[string]any, error) {
	panic("TransformToAuthProvider Unimplemented!")
}

func (p *mockLdapProvider) UsesUserSecrets() bool      { return false }
func (p *mockLdapProvider) CanRefreshPrincipals() bool { return true }

func (p *mockLdapProvider) RefetchGroupPrincipals(principalID string, secret string) ([]apiv3.Principal, error) {
	panic("RefetchGroupPrincipals Unimplemented!")
}

func (p *mockLdapProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []apiv3.Principal) (bool, error) {
	panic("CanAccessWithGroupProviders Unimplemented!")
}

func (p *mockLdapProvider) GetUserExtraAttributes(userPrincipal apiv3.Principal) map[string][]string {
	panic("GetUserExtraAttributes Unimplemented!")
}

func (p *mockLdapProvider) GetUserExtraAttributesFromToken(token accessor.TokenAccessor) map[string][]string {
	panic("GetUserExtraAttributesFromToken Unimplemented!")
}

func (p *mockLdapProvider) IsDisabledProvider(string) (bool, error) {
	panic("IsDisabledProvider Unimplemented!")
}

func TestSearchPrincipalsResolvesKnownUsers(t *testing.T) {
	t.Parallel()

	users := []*apiv3.User{
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-ping1"},
			DisplayName:  "Test UserOne",
			PrincipalIDs: []string{"ping-eu_user://uid-0001", "local://u-ping1"},
		},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-okta1"},
			DisplayName:  "Test UserFromOkta",
			PrincipalIDs: []string{"okta_user://abc-uuid-1", "local://u-okta1"},
		},
	}

	provider := &Provider{
		name: PingName,
		userSearcher: common.NewUserSearcher(&fakes.UserListerMock{
			ListFunc: func(namespace string, selector labels.Selector) ([]*apiv3.User, error) {
				return users, nil
			},
		}),
	}

	tests := []struct {
		name          string
		searchKey     string
		principalType string
		want          []apiv3.Principal
	}{
		{
			name:          "known user is returned before the principal built from the search key",
			searchKey:     "testu",
			principalType: common.UserPrincipalType,
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_user://uid-0001"},
					DisplayName:   "Test UserOne",
					PrincipalType: common.UserPrincipalType,
					Provider:      PingName,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_user://testu"},
					DisplayName:   "testu",
					LoginName:     "testu",
					PrincipalType: common.UserPrincipalType,
					Provider:      PingName,
				},
			},
		},
		{
			name:      "known user is returned alongside the group principal",
			searchKey: "testu",
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_user://uid-0001"},
					DisplayName:   "Test UserOne",
					PrincipalType: common.UserPrincipalType,
					Provider:      PingName,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_user://testu"},
					DisplayName:   "testu",
					LoginName:     "testu",
					PrincipalType: common.UserPrincipalType,
					Provider:      PingName,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_group://testu"},
					DisplayName:   "testu",
					LoginName:     "testu",
					PrincipalType: common.GroupPrincipalType,
					Provider:      PingName,
				},
			},
		},
		{
			name:          "users of another provider are not returned",
			searchKey:     "UserFromOkta",
			principalType: common.UserPrincipalType,
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_user://UserFromOkta"},
					DisplayName:   "UserFromOkta",
					LoginName:     "UserFromOkta",
					PrincipalType: common.UserPrincipalType,
					Provider:      PingName,
				},
			},
		},
		{
			name:          "searching the external id returns a single principal",
			searchKey:     "uid-0001",
			principalType: common.UserPrincipalType,
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_user://uid-0001"},
					DisplayName:   "uid-0001",
					LoginName:     "uid-0001",
					PrincipalType: common.UserPrincipalType,
					Provider:      PingName,
				},
			},
		},
		{
			name:          "group search does not resolve users",
			searchKey:     "testu",
			principalType: common.GroupPrincipalType,
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_group://testu"},
					DisplayName:   "testu",
					LoginName:     "testu",
					PrincipalType: common.GroupPrincipalType,
					Provider:      PingName,
				},
			},
		},
	}

	token := &apiv3.Token{
		AuthProvider: "ping-eu",
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ping-eu_user://00ux3opnquJigzHYx697",
			},
			LoginName:     "developer",
			PrincipalType: "user",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := provider.SearchPrincipals(test.searchKey, test.principalType, token)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestSearchPrincipalsWithoutUserSearcher(t *testing.T) {
	t.Parallel()

	provider := &Provider{
		name: PingName,
	}

	token := &apiv3.Token{
		AuthProvider: PingName,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ping_user://00ux3opnquJigzHYx697",
			},
			LoginName:     "developer",
			PrincipalType: "user",
		},
	}

	got, err := provider.SearchPrincipals("testu", common.UserPrincipalType, token)
	require.NoError(t, err)
	assert.Equal(t, []apiv3.Principal{
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "ping_user://testu"},
			DisplayName:   "testu",
			LoginName:     "testu",
			PrincipalType: common.UserPrincipalType,
			Provider:      PingName,
		},
	}, got)
}

func TestSearchPrincipalsUserSearchError(t *testing.T) {
	t.Parallel()

	provider := &Provider{
		name: PingName,
		userSearcher: common.NewUserSearcher(&fakes.UserListerMock{
			ListFunc: func(namespace string, selector labels.Selector) ([]*apiv3.User, error) {
				return nil, errors.New("cache is not synced")
			},
		}),
	}

	token := &apiv3.Token{
		AuthProvider: "ping-eu",
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ping_user://00ux3opnquJigzHYx697",
			},
			LoginName:     "developer",
			PrincipalType: "user",
		},
	}

	got, err := provider.SearchPrincipals("testu", common.UserPrincipalType, token)
	require.ErrorContains(t, err, "cache is not synced")
	assert.Nil(t, got)
}

func TestFormSamlRedirectURLGenericSAML(t *testing.T) {
	cfg := map[string]any{
		client.GenericSAMLConfigFieldRancherAPIHost: "https://rancher.example.com",
	}
	got := formSamlRedirectURLFromMap(cfg, GenericSAMLName)
	assert.Equal(t, "https://rancher.example.com/v1-saml/genericsaml/login", got)
}

func TestTransformToAuthProviderGenericSAML(t *testing.T) {
	p := &Provider{name: GenericSAMLName}
	out, err := p.TransformToAuthProvider(map[string]any{
		client.GenericSAMLConfigFieldRancherAPIHost: "https://rancher.example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://rancher.example.com/v1-saml/genericsaml/login",
		out[publicclient.GenericSAMLProviderFieldRedirectURL])
}

func TestTransformToAuthProvider(t *testing.T) {
	authConfig := map[string]any{
		client.OKTAConfigFieldRancherAPIHost: "https://rancher.example.com",
		"metadata": map[string]any{
			"name": "okta-1",
		},
	}

	p := &Provider{name: OKTAName}
	out, err := p.TransformToAuthProvider(authConfig)
	require.NoError(t, err)
	assert.Equal(t, "https://rancher.example.com/v1-saml/okta-1/login",
		out[publicclient.OKTAProviderFieldRedirectURL])
}

func TestLogoutUsesConfigFromToken(t *testing.T) {
	const configName = "ping-eu"

	for _, name := range []string{PingName, configName} {
		original, exists := SamlProviders[name]
		t.Cleanup(func() {
			if exists {
				SamlProviders[name] = original
				return
			}
			delete(SamlProviders, name)
		})
	}
	SamlProviders[PingName] = &Provider{name: PingName, sloForced: false}
	SamlProviders[configName] = &Provider{name: PingName, sloForced: true}

	// The token records the provider name, but logout must use the settings of
	// the config that issued it.
	token := &apiv3.Token{
		AuthProvider: PingName,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: configName + "_user://testu"},
			Provider:   PingName,
		},
	}

	p := &Provider{name: PingName}
	err := p.Logout(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil), token)
	assert.ErrorContains(t, err, "configured for forced SLO")
}

func TestPerformSamlLoginUsesConfigName(t *testing.T) {
	newProvider := func(configName string) (*Provider, *recordingClientState) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		clientState := newRecordingClientState()

		return &Provider{
			name:        OKTAName,
			clientState: clientState,
			serviceProvider: &saml.ServiceProvider{
				Key:         privateKey,
				MetadataURL: testParseURL(t, "https://rancher.example.com/v1-saml/"+configName+"/saml/metadata"),
				AcsURL:      testParseURL(t, "https://rancher.example.com/v1-saml/"+configName+"/saml/acs"),
				IDPMetadata: &saml.EntityDescriptor{
					IDPSSODescriptors: []saml.IDPSSODescriptor{{
						SingleSignOnServices: []saml.Endpoint{{
							Binding:  saml.HTTPRedirectBinding,
							Location: "https://idp.example.com/sso",
						}},
					}},
				},
			},
		}, clientState
	}

	setupSamlProviderTypes(t, OKTAName)
	okta, oktaState := newProvider("okta")
	oktaEU, oktaEUState := newProvider("okta-eu")
	setSamlProvider("okta", okta)
	setSamlProvider("okta-eu", oktaEU)

	loginInput := &apiv3.SamlLoginInput{
		GenericLogin:     apiv3.GenericLogin{Name: OKTAName, ConfigName: "okta-eu"},
		FinalRedirectURL: "https://rancher.example.com/dashboard",
	}
	req := httptest.NewRequest(http.MethodPost, "/v3-public/oktaProviders/okta?action=login", nil)

	err := PerformSamlLogin(req, httptest.NewRecorder(), OKTAName, loginInput, samlProviderTypes[OKTAName])
	require.NoError(t, err)

	assert.Equal(t, "/v1-saml/okta-eu/saml/acs", oktaEUState.path)
	assert.Equal(t, loginAction, oktaEUState.states["Rancher_Action"])
	assert.Empty(t, oktaState.states, "the state for other configs should not be used")
}

func TestPerformSamlLoginNotInitialized(t *testing.T) {
	setupSamlProviderTypes(t, OKTAName)
	// The base provider is registered for the default config before the
	// config is initialized.
	setSamlProvider(OKTAName, samlProviderTypes[OKTAName])

	loginInput := &apiv3.SamlLoginInput{FinalRedirectURL: "https://rancher.example.com/dashboard"}
	req := httptest.NewRequest(http.MethodPost, "/v3-public/oktaProviders/okta?action=login", nil)

	err := PerformSamlLogin(req, httptest.NewRecorder(), OKTAName, loginInput, samlProviderTypes[OKTAName])
	assert.ErrorContains(t, err, "not initialized")

	err = PerformSamlLogin(req, httptest.NewRecorder(), OKTAName, &apiv3.SamlLoginInput{
		GenericLogin:     apiv3.GenericLogin{ConfigName: "okta-eu"},
		FinalRedirectURL: "https://rancher.example.com/dashboard",
	}, samlProviderTypes[OKTAName])
	assert.ErrorContains(t, err, "okta-eu not initialized")
}

func TestGetPrincipal(t *testing.T) {
	provider := &Provider{name: PingName}
	token := &apiv3.Token{
		UserPrincipal: apiv3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_user://alice"},
			DisplayName:   "Alice",
			LoginName:     "alice@example.com",
			PrincipalType: common.UserPrincipalType,
		},
	}

	tests := []struct {
		name        string
		principalID string
		token       accessor.TokenAccessor
		want        apiv3.Principal
		wantErr     bool
	}{
		{
			name:        "user principal for the default config",
			principalID: "ping_user://bob",
			want: apiv3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: "ping_user://bob"},
				DisplayName:   "bob",
				LoginName:     "bob",
				PrincipalType: common.UserPrincipalType,
				Provider:      PingName,
			},
		},
		{
			name:        "user principal for another config than the token",
			principalID: "ping-us_user://bob",
			token:       token,
			want: apiv3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: "ping-us_user://bob"},
				DisplayName:   "bob",
				LoginName:     "bob",
				PrincipalType: common.UserPrincipalType,
				Provider:      PingName,
			},
		},
		{
			name:        "user principal matching the token",
			principalID: "ping-eu_user://alice",
			token:       token,
			want: apiv3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_user://alice"},
				DisplayName:   "Alice",
				LoginName:     "alice@example.com",
				PrincipalType: common.UserPrincipalType,
				Provider:      PingName,
				Me:            true,
			},
		},
		{
			name:        "group principal for an additional config",
			principalID: "ping-eu_group://admins",
			want: apiv3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: "ping-eu_group://admins"},
				DisplayName:   "admins",
				LoginName:     "admins",
				PrincipalType: common.GroupPrincipalType,
				Provider:      PingName,
			},
		},
		{
			name:        "invalid principal type",
			principalID: "ping-eu_other://admins",
			wantErr:     true,
		},
		{
			name:        "invalid principal ID",
			principalID: "admins",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.GetPrincipal(tt.principalID, tt.token)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLogoutAllowedForRemovedConfig(t *testing.T) {
	setupSamlProviderTypes(t, PingName)

	token := &apiv3.Token{
		AuthProvider: PingName,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: "ping-eu_user://testu"},
			Provider:   PingName,
		},
	}

	p := &Provider{name: PingName}
	err := p.Logout(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil), token)
	assert.NoError(t, err, "SLO can't be forced for a config that is disabled or deleted")
}
