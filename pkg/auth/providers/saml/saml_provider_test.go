package saml

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/rancher/norman/types"
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

func TestConfiguredOktaProviderContainsLdapProvider(t *testing.T) {
	// saml.Configure runs some ldap specific logic based on the saml provider name, so we provide
	// just enough scaffolding to run the Configure function.
	ctx := context.Background()
	mgmtCtx, err := config.NewScaledContext(rest.Config{}, nil)
	require.NoError(t, err, "Failed to create NewScaledContext")

	// Create the dummy wrangler context
	wranglerContext, err := wrangler.NewContext(ctx, nil, &rest.Config{})
	require.NoError(t, err, "Failed to create wranglerContext")
	mgmtCtx.Wrangler = wranglerContext

	tokenMGR := tokens.NewManager(wranglerContext)
	provider, ok := Configure(mgmtCtx, mgmtCtx.UserManager, tokenMGR, "okta").(*Provider)
	require.True(t, ok, "Failed to Configure a valid Provider")

	assert.True(t, provider.hasLdapGroupSearch(), "Missing LDAP group search capability for okta provider")
	assert.NotNil(t, provider.ldapProvider, "Configured okta provider did not receive child LDAP provider")
}

func TestSearchPrincipals(t *testing.T) {
	providerName := "okta"
	userType := "okta_user"
	groupType := "okta_group"

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

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			provider := &Provider{
				name:      providerName,
				userType:  userType,
				groupType: groupType,
				ldapProvider: &mockLdapProvider{
					providerName:     providerName,
					isLdapConfigured: tt.isLdapConfigured,
				},
			}

			results, err := provider.SearchPrincipals(tt.searchKey, tt.principalType, &apiv3.Token{})
			require.NoError(t, err)
			require.Len(t, results, len(tt.principals))
			for _, principal := range results {
				assert.Contains(t, tt.principals, principal.Name)
			}
		})

		// same behaviour for ext tokens
		t.Run(tt.desc+", ext", func(t *testing.T) {
			provider := &Provider{
				name:      providerName,
				userType:  userType,
				groupType: groupType,
				ldapProvider: &mockLdapProvider{
					providerName:     providerName,
					isLdapConfigured: tt.isLdapConfigured,
				},
			}

			results, err := provider.SearchPrincipals(tt.searchKey, tt.principalType, &ext.Token{})
			require.NoError(t, err)
			require.Len(t, results, len(tt.principals))
			for _, principal := range results {
				assert.Contains(t, tt.principals, principal.Name)
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

var _ accessor.TokenAccessor = (*fakeToken)(nil)

type fakeToken struct {
	authProvider string
}

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

func (p *mockLdapProvider) IsDisabledProvider() (bool, error) {
	panic("IsDisabledProvider Unimplemented!")
}
