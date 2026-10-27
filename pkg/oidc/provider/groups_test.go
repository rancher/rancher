package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	providermocks "github.com/rancher/rancher/pkg/auth/providers/mocks"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/oidc/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	groupsTestUserID = "u-groups"
	groupsTestKID    = "groups-kid"
)

// newGroupsTestUserAttribute returns a UserAttribute with groups from two
// AuthConfigs, as a user who has logged in through both has.
func newGroupsTestUserAttribute() *v3.UserAttribute {
	group := func(name string) v3.Principals {
		return v3.Principals{Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: name}}}}
	}
	return &v3.UserAttribute{
		GroupPrincipals: map[string]v3.Principals{
			"github-eu": group("github-eu_team://10"),
			"ldap-corp": group("ldap-corp_group://cn=admins,dc=corp"),
		},
	}
}

// registerGroupsTestProviders registers enabled github-eu (github) and
// ldap-corp (openldap) AuthConfigs.
func registerGroupsTestProviders(t *testing.T, ctrl *gomock.Controller) {
	t.Helper()

	githubProvider := providermocks.NewMockAuthProvider(ctrl)
	githubProvider.EXPECT().IsDisabledProvider("github-eu").Return(false, nil).AnyTimes()
	providers.SetProviders(map[string]common.AuthProvider{"github": githubProvider})
	providers.SetAuthConfigLister(&fakes.AuthConfigListerMock{
		GetFunc: func(_ string, name string) (*v3.AuthConfig, error) {
			switch name {
			case "github-eu":
				return &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: client.GithubConfigType}, nil
			case "ldap-corp":
				return &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: client.OpenLdapConfigType}, nil
			}
			return nil, errors.NewNotFound(v3.Resource("authconfigs"), name)
		},
	})
	t.Cleanup(func() {
		providers.SetProviders(nil)
		providers.SetAuthConfigLister(nil)
	})
}

func newGroupsTestToken() *v3.Token {
	return &v3.Token{
		ObjectMeta:    metav1.ObjectMeta{Name: "token-groups"},
		UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github-eu_user://1"}},
		UserID:        groupsTestUserID,
	}
}

// The groups claim of the ID token must only hold the groups of the AuthConfig
// that issued the Rancher token, as Authenticate does for Rancher requests.
func TestTokenEndpointGroupsFromTokenAuthConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	registerGroupsTestProviders(t, ctrl)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := mocks.NewMocksigningKeyGetter(ctrl)
	jwks.EXPECT().GetSigningKey().Return(privateKey, groupsTestKID, nil)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(groupsTestUserID).Return(&v3.User{DisplayName: "Alice"}, nil)
	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(groupsTestUserID).Return(newGroupsTestUserAttribute(), nil)
	h := &tokenHandler{
		userLister:          userCache,
		userAttributeLister: userAttributeCache,
		jwks:                jwks,
		now:                 time.Now,
	}
	oidcClient := &v3.OIDCClient{
		Spec:   v3.OIDCClientSpec{TokenExpirationSeconds: 600},
		Status: v3.OIDCClientStatus{ClientID: "client-id"},
	}

	resp, oidcErr := h.createTokenResponse(newGroupsTestToken(), oidcClient, "", []string{"openid", "groups"})
	require.Nil(t, oidcErr)

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(resp.IDToken, &claims, func(*jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	require.NoError(t, err)
	assert.Equal(t, []any{"github-eu_team://10"}, claims["groups"])
}

// The userinfo groups must only hold the groups of the AuthConfig that issued
// the Rancher token the access token was created for.
func TestUserInfoEndpointGroupsFromTokenAuthConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	registerGroupsTestProviders(t, ctrl)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	oidcClient := &v3.OIDCClient{
		Spec:   v3.OIDCClientSpec{TokenExpirationSeconds: 600},
		Status: v3.OIDCClientStatus{ClientID: "client-id"},
	}
	accessToken, err := CreateAccessToken(oidcClient, newGroupsTestToken(), []string{"openid", "groups"}, groupsTestKID, time.Now()).
		SignedString(privateKey)
	require.NoError(t, err)

	jwks := mocks.NewMocksigningKeyGetter(ctrl)
	jwks.EXPECT().GetPublicKey(groupsTestKID).Return(&privateKey.PublicKey, nil)
	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(groupsTestUserID).Return(newGroupsTestUserAttribute(), nil)
	h := userInfoHandler{
		userCache:           fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl),
		userAttributeLister: userAttributeCache,
		jwks:                jwks,
	}
	req := httptest.NewRequest(http.MethodGet, "https://rancher.example.com/oidc/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	h.userInfoEndpoint(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response UserInfoResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, []string{"github-eu_team://10"}, response.Groups)
}

func TestGroupsForConfig(t *testing.T) {
	tests := map[string]struct {
		attribs    *v3.UserAttribute
		configName string
		want       []string
	}{
		"groups of the config": {
			attribs:    newGroupsTestUserAttribute(),
			configName: "github-eu",
			want:       []string{"github-eu_team://10"},
		},
		"the local config has no groups": {
			attribs:    newGroupsTestUserAttribute(),
			configName: "local",
		},
		"no config name": {
			attribs: newGroupsTestUserAttribute(),
		},
		"no user attribute": {
			configName: "github-eu",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, groupsForConfig(tt.attribs, tt.configName))
		})
	}
}
