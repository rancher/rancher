package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/oidc/mocks"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestUserInfoEndpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	type mockParams struct {
		userCache          *fake.MockNonNamespacedCacheInterface[*v3.User]
		useAttributeLister *fake.MockNonNamespacedCacheInterface[*v3.UserAttribute]
		signingKeyGetter   *mocks.MocksigningKeyGetter
	}
	const (
		fakeSigningKey = "signing-key"
		fakeUserName   = "user-name"
		fakeUserID     = "user-id"
		fakeGroupName  = "github-eu_team://10"
		fakeConfigName = "github-eu"

		fakeRancherTokenName = "token-abc12"
	)
	fakeRancherToken := &v3.Token{
		ObjectMeta:    metav1.ObjectMeta{Name: fakeRancherTokenName},
		UserID:        fakeUserID,
		UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github-eu_user://1"}, Provider: "github"},
	}
	fakeUser := v3.User{
		DisplayName: fakeUserName,
	}
	fakeUserAttributes := v3.UserAttribute{
		GroupPrincipals: map[string]v3.Principals{
			fakeConfigName: {
				Items: []v3.Principal{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeGroupName,
						},
					},
				},
			},
			// Groups from another AuthConfig must not be returned.
			"github": {
				Items: []v3.Principal{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "github_team://other-group",
						},
					},
				},
			},
		},
	}
	fakeAccessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":           []interface{}{"client-id"},
		"exp":           float64(time.Now().Add(10 * time.Hour).Unix()),
		"iss":           settings.ServerURL.Get() + "/oidc",
		"iat":           float64(time.Now().Unix()),
		"sub":           fakeUserID,
		configNameClaim: fakeConfigName,
		"scope":         []string{"openid", "profile", "groups"},
	})
	fakeAccessToken.Header["kid"] = fakeSigningKey
	var privateKey *rsa.PrivateKey
	privateKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	fakeAccessTokenString, _ := fakeAccessToken.SignedString(privateKey)
	fakeAccessTokenNoProfile := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":           []interface{}{"client-id"},
		"exp":           float64(time.Now().Add(10 * time.Hour).Unix()),
		"iss":           settings.ServerURL.Get() + "/oidc",
		"iat":           float64(time.Now().Unix()),
		"sub":           fakeUserID,
		configNameClaim: fakeConfigName,
		"scope":         []string{"openid", "groups"},
	})
	fakeAccessTokenNoProfile.Header["kid"] = fakeSigningKey
	fakeAccessTokenNoProfileString, _ := fakeAccessTokenNoProfile.SignedString(privateKey)

	fakeAccessTokenNoGroups := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":           []interface{}{"client-id"},
		"exp":           float64(time.Now().Add(10 * time.Hour).Unix()),
		"iss":           settings.ServerURL.Get() + "/oidc",
		"iat":           float64(time.Now().Unix()),
		"sub":           fakeUserID,
		"auth_provider": "auth-provider",
		"scope":         []string{"openid", "profile"},
	})
	fakeAccessTokenNoGroups.Header["kid"] = fakeSigningKey
	fakeAccessTokenNoGroupsString, _ := fakeAccessTokenNoGroups.SignedString(privateKey)

	// Access tokens issued while both claims were written. auth_provider holds
	// the provider type, which matches the config "github" that the user didn't
	// log in through.
	fakeAccessTokenBothClaims := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":           []interface{}{"client-id"},
		"exp":           float64(time.Now().Add(10 * time.Hour).Unix()),
		"iss":           settings.ServerURL.Get() + "/oidc",
		"iat":           float64(time.Now().Unix()),
		"sub":           fakeUserID,
		"auth_provider": "github",
		configNameClaim: fakeConfigName,
		"scope":         []string{"openid", "profile", "groups"},
	})
	fakeAccessTokenBothClaims.Header["kid"] = fakeSigningKey
	fakeAccessTokenBothClaimsString, _ := fakeAccessTokenBothClaims.SignedString(privateKey)

	// Access tokens issued before the config_name claim was added. The user
	// logged in through github-eu, but auth_provider holds the provider type.
	fakeAccessTokenNoConfigName := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":           []interface{}{"client-id"},
		"exp":           float64(time.Now().Add(10 * time.Hour).Unix()),
		"iss":           settings.ServerURL.Get() + "/oidc",
		"iat":           float64(time.Now().Unix()),
		"sub":           fakeUserID,
		"auth_provider": "github",
		"token":         fakeRancherTokenName,
		"token_kind":    "v3",
		"scope":         []string{"openid", "profile", "groups"},
	})
	fakeAccessTokenNoConfigName.Header["kid"] = fakeSigningKey
	fakeAccessTokenNoConfigNameString, _ := fakeAccessTokenNoConfigName.SignedString(privateKey)

	// Refresh tokens issued before the upgrade have auth_provider but don't
	// name the Rancher token.
	fakeRefreshTokenWithAuthProvider := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":                []interface{}{"client-id"},
		"exp":                float64(time.Now().Add(10 * time.Hour).Unix()),
		"iat":                float64(time.Now().Unix()),
		"sub":                fakeUserID,
		"auth_provider":      "github",
		"rancher_token_hash": "hash",
		"scope":              []string{"openid", "profile", "groups"},
	})
	fakeRefreshTokenWithAuthProvider.Header["kid"] = fakeSigningKey
	fakeRefreshTokenWithAuthProviderString, _ := fakeRefreshTokenWithAuthProvider.SignedString(privateKey)

	fakeAccessTokenNoConfigNameOrProvider := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":   []interface{}{"client-id"},
		"exp":   float64(time.Now().Add(10 * time.Hour).Unix()),
		"iss":   settings.ServerURL.Get() + "/oidc",
		"iat":   float64(time.Now().Unix()),
		"sub":   fakeUserID,
		"scope": []string{"openid", "profile", "groups"},
	})
	fakeAccessTokenNoConfigNameOrProvider.Header["kid"] = fakeSigningKey
	fakeAccessTokenNoConfigNameOrProviderString, _ := fakeAccessTokenNoConfigNameOrProvider.SignedString(privateKey)

	tests := map[string]struct {
		req          func() *http.Request
		mockSetup    func(mockParams)
		tokens       map[string]accessor.TokenAccessor
		tokenErr     error
		wantResponse *UserInfoResponse
		wantError    string
		wantStatus   int
		wantHeaders  map[string]string
	}{
		"success response": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
				Groups:   []string{fakeGroupName},
			},
		},
		"success response without profile": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenNoProfileString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			wantResponse: &UserInfoResponse{
				Sub:    fakeUserID,
				Groups: []string{fakeGroupName},
			},
		},
		"success response without groups": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenNoGroupsString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
			},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
			},
		},
		"access token with config name and auth provider gets the groups of the config name": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenBothClaimsString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
				Groups:   []string{fakeGroupName},
			},
		},
		"access token without config name gets the groups of its rancher token's config, not of its auth provider": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenNoConfigNameString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			tokens: map[string]accessor.TokenAccessor{fakeRancherTokenName: fakeRancherToken},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
				Groups:   []string{fakeGroupName},
			},
		},
		"access token without config name whose rancher token is gone gets no groups": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenNoConfigNameString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
			},
		},
		"access token without config name whose rancher token belongs to another user gets no groups": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenNoConfigNameString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			tokens: map[string]accessor.TokenAccessor{fakeRancherTokenName: &v3.Token{
				ObjectMeta:    metav1.ObjectMeta{Name: fakeRancherTokenName},
				UserID:        "another-user",
				UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github-eu_user://2"}, Provider: "github"},
			}},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
			},
		},
		"refresh token with auth provider gets no groups": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeRefreshTokenWithAuthProviderString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
			},
		},
		"access token without config name fails if its rancher token can't be fetched": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenNoConfigNameString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			tokenErr:   errors.New("unexpected error"),
			wantError:  `{"error":"server_error","error_description":"can't get rancher token: unexpected error"}`,
			wantStatus: http.StatusInternalServerError,
		},
		"access token without config name or auth provider gets no groups": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenNoConfigNameOrProviderString))

				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
			},
		},
		"invalid signature": {
			req: func() *http.Request {
				anotherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				accessTokenString, _ := fakeAccessToken.SignedString(anotherKey)
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessTokenString))
				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
			},
			wantError:  `{"error":"invalid_request","error_description":"invalid access_token: token signature is invalid: crypto/rsa: verification error"}`,
			wantStatus: http.StatusBadRequest,
		},
		"invalid access token": {
			req: func() *http.Request {
				invalidAccessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
					"aud":           []interface{}{"client-id"},
					"exp":           float64(time.Now().Add(10 * time.Hour).Unix()),
					"iss":           settings.ServerURL.Get() + "/oidc",
					"iat":           float64(time.Now().Unix()),
					"sub":           fakeUserID,
					"auth_provider": "auth-provider",
				})
				invalidAccessToken.Header["kid"] = fakeSigningKey
				accessTokenString, _ := invalidAccessToken.SignedString(privateKey)
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessTokenString))
				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
			},
			wantError:  `{"error":"invalid_request","error_description":"invalid access_token: it doesn't have scope"}`,
			wantStatus: http.StatusBadRequest,
		},
		"no access token": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				return req
			},
			wantError:  `{"error":"invalid_request","error_description":"failed to get token from header: authorization header is missing"}`,
			wantStatus: http.StatusBadRequest,
		},
		"userinfo endpoint sets cache-control headers": {
			req: func() *http.Request {
				req, _ := http.NewRequest(http.MethodGet, "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fakeAccessTokenString))
				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				mockParams.userCache.EXPECT().Get(fakeUserID).Return(&fakeUser, nil)
				mockParams.useAttributeLister.EXPECT().Get(fakeUserID).Return(&fakeUserAttributes, nil)
			},
			wantHeaders: map[string]string{
				"Cache-Control": "no-store",
				"Pragma":        "no-cache",
			},
			wantResponse: &UserInfoResponse{
				Sub:      fakeUserID,
				UserName: fakeUserName,
				Groups:   []string{fakeGroupName},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m := mockParams{
				userCache:          fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl),
				useAttributeLister: fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl),
				signingKeyGetter:   mocks.NewMocksigningKeyGetter(ctrl),
			}
			if test.mockSetup != nil {
				test.mockSetup(m)
			}
			h := userInfoHandler{
				userCache:           m.userCache,
				userAttributeLister: m.useAttributeLister,
				jwks:                m.signingKeyGetter,
				tokenFetcher:        &fakeTokenFetcher{tokens: test.tokens, err: test.tokenErr},
			}
			rec := httptest.NewRecorder()

			h.userInfoEndpoint(rec, test.req())

			for k, v := range test.wantHeaders {
				assert.Equal(t, v, rec.Header().Get(k), "response header %s", k)
			}
			if test.wantError != "" {
				assert.Equal(t, test.wantStatus, rec.Code, "unexpected status code")
				assert.JSONEq(t, test.wantError, strings.TrimSpace(rec.Body.String()))
			} else {
				var response UserInfoResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, test.wantResponse, &response)
			}
		})
	}
}

// fakeTokenFetcher returns the named token, err if set, or a NotFound error.
type fakeTokenFetcher struct {
	tokens map[string]accessor.TokenAccessor
	err    error
}

func (f *fakeTokenFetcher) Fetch(name string) (accessor.TokenAccessor, error) {
	if f.err != nil {
		return nil, f.err
	}
	if token, ok := f.tokens[name]; ok {
		return token, nil
	}

	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "tokens"}, name)
}
