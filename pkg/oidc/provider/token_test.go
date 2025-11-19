package provider

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/oidc/provider/session"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/golang-jwt/jwt/v5"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	providermocks "github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/oidc/mocks"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestTokenEndpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	type mockParams struct {
		tokenCache         *fake.MockNonNamespacedCacheInterface[*v3.Token]
		tokenClient        *fake.MockNonNamespacedClientInterface[*v3.Token, *v3.TokenList]
		secretCache        *fake.MockCacheInterface[*v1.Secret]
		oidcClientCache    *fake.MockNonNamespacedCacheInterface[*v3.OIDCClient]
		oidcClient         *fake.MockNonNamespacedClientInterface[*v3.OIDCClient, *v3.OIDCClientList]
		userLister         *fake.MockNonNamespacedCacheInterface[*v3.User]
		useAttributeLister *fake.MockNonNamespacedCacheInterface[*v3.UserAttribute]
		sessionClient      *mocks.MocksessionGetterRemover
		signingKeyGetter   *mocks.MocksigningKeyGetter
	}
	const (
		fakeCode                 = "code123"
		fakeClientID             = "client-id"
		fakeClientName           = "client-name"
		fakeClientSecret         = "client-secret"
		fakeCodeVerifier         = "code-verifier"
		fakeTokenName            = "token-name"
		fakeUserID               = "user-id"
		fakeAuthProvider         = "auth-provider"
		fakeUsername             = "username"
		fakeGroup                = "group"
		fakeSigningKey           = "key"
		fakeClientSecretID       = "client-secret-1"
		fakeTokenLifespan        = 600
		fakeRefreshTokenLifespan = 3600
	)

	fakeScopes := []any{"openid", "profile"}
	fakeScopesOfflineAccess := []any{"openid", "profile", "offline_access"}
	now := time.Now()
	fakeTime := func() time.Time {
		return now
	}
	var privateKey *rsa.PrivateKey
	fakeSession := &session.Session{
		ClientID:      fakeClientID,
		TokenName:     fakeTokenName,
		Scope:         []string{"openid", "profile"},
		CodeChallenge: oauth2.S256ChallengeFromVerifier(fakeCodeVerifier),
	}
	fakeSessionOfflineAccess := &session.Session{
		ClientID:      fakeClientID,
		TokenName:     fakeTokenName,
		Scope:         []string{"openid", "profile", "offline_access"},
		CodeChallenge: oauth2.S256ChallengeFromVerifier(fakeCodeVerifier),
	}
	fakeOIDCClient := &v3.OIDCClient{
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeClientName,
		},
		Spec: v3.OIDCClientSpec{
			TokenExpirationSeconds:        fakeTokenLifespan,
			RefreshTokenExpirationSeconds: fakeRefreshTokenLifespan,
		},
		Status: v3.OIDCClientStatus{
			ClientID: fakeClientID,
		},
	}

	fakeToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeTokenName,
		},
		Token:        "this-is-a-test",
		UserID:       fakeUserID,
		Enabled:      ptr.To(true),
		AuthProvider: fakeAuthProvider,
	}
	fakeExpiredToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeTokenName,
			CreationTimestamp: metav1.Time{
				Time: time.Unix(10, 0),
			},
		},
		Token:        "this-is-a-test",
		UserID:       fakeUserID,
		Enabled:      ptr.To(true),
		AuthProvider: fakeAuthProvider,
		TTLMillis:    1,
	}
	fakeUser := &v3.User{
		DisplayName: fakeUsername,
		Enabled:     ptr.To(true),
	}
	fakeUserAttributes := &v3.UserAttribute{
		GroupPrincipals: map[string]v3.Principals{
			"group": {
				Items: []v3.Principal{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeGroup,
						},
					},
				},
			},
		},
	}
	fakeTokenList := []*v3.Token{fakeToken}
	fakeTokenExpiredList := []*v3.Token{fakeExpiredToken}
	hash := sha256.Sum256([]byte(fakeTokenName))
	rancherTokenHash := hex.EncodeToString(hash[:])
	fakeRefreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":                []string{fakeClientID},
		"exp":                now.Add(10 * time.Hour).Unix(),
		"iat":                now.Unix(),
		"sub":                fakeUserID,
		"rancher_token_hash": rancherTokenHash,
		"scope":              fakeScopesOfflineAccess,
	})
	fakeRefreshToken.Header["kid"] = fakeSigningKey
	privateKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	fakeRefreshTokenString, _ := fakeRefreshToken.SignedString(privateKey)
	fakeClientk8sSecret := &v1.Secret{
		Data: map[string][]byte{
			fakeClientSecretID: []byte(fakeClientSecret),
		},
	}
	clientSecretIDPatch, _ := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:   "add",
		Path: "/metadata/annotations",
		Value: map[string]string{
			"cattle.io.oidc-client-secret-used-" + fakeClientSecretID: fmt.Sprintf("%d", fakeTime().Unix()),
		},
	}})
	tokenPatch, _ := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:   "add",
		Path: "/metadata/labels",
		Value: map[string]string{
			"cattle.io.oidc-client-" + fakeClientName: "true",
		},
	}})
	tests := map[string]struct {
		req                    func() *http.Request
		mockSetup              func(mockParams)
		wantIdTokenClaims      *jwt.MapClaims
		wantAccessTokenClaims  *jwt.MapClaims
		wantRefreshTokenClaims *jwt.MapClaims
		wantError              string
	}{
		"authorization_code returns an id_token and access_token": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "authorization_code")
				data.Set("code", fakeCode)
				data.Set("code_verifier", fakeCodeVerifier)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.sessionClient.EXPECT().Get(fakeCode).Return(fakeSession, nil)
				m.sessionClient.EXPECT().Remove(fakeCode).Return(nil)
				m.secretCache.EXPECT().Get("cattle-oidc-client-secrets", fakeClientID).Return(fakeClientk8sSecret, nil)
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{fakeOIDCClient}, nil)
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(fakeToken, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(fakeUser, nil)
				m.useAttributeLister.EXPECT().Get(fakeUserID).Return(fakeUserAttributes, nil)
				m.signingKeyGetter.EXPECT().GetSigningKey().Return(privateKey, fakeSigningKey, nil)
				m.oidcClient.EXPECT().Patch(fakeClientName, types.JSONPatchType, clientSecretIDPatch).Return(fakeOIDCClient, nil)
			},
			wantIdTokenClaims: &jwt.MapClaims{
				"aud":           []any{fakeClientID},
				"exp":           float64(fakeTime().Add(fakeTokenLifespan * time.Second).Unix()),
				"iss":           settings.ServerURL.Get() + "/oidc",
				"iat":           float64(fakeTime().Unix()),
				"name":          fakeUsername,
				"sub":           fakeUserID,
				"auth_provider": fakeAuthProvider,
				"groups":        []any{fakeGroup},
			},
			wantAccessTokenClaims: &jwt.MapClaims{
				"aud":           []any{fakeClientID},
				"exp":           float64(fakeTime().Add(fakeTokenLifespan * time.Second).Unix()),
				"iss":           settings.ServerURL.Get() + "/oidc",
				"iat":           float64(fakeTime().Unix()),
				"sub":           fakeUserID,
				"auth_provider": fakeAuthProvider,
				"scope":         fakeScopes,
				"token":         fakeToken.Name + ":" + fakeToken.Token,
			},
		},
		"authorization_code fails for an invalid code": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "authorization_code")
				data.Set("code", fakeCode)
				data.Set("code_verifier", fakeCodeVerifier)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.sessionClient.EXPECT().Get(fakeCode).Return(nil, errors.NewNotFound(schema.GroupResource{}, "secret not found"))
			},
			wantError: `{"error":"invalid_request","error_description":"invalid code"}`,
		},
		"authorization_code fails for an invalid client secret": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "authorization_code")
				data.Set("code", fakeCode)
				data.Set("code_verifier", fakeCodeVerifier)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.sessionClient.EXPECT().Get(fakeCode).Return(fakeSession, nil)
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{fakeOIDCClient}, nil)
				m.secretCache.EXPECT().Get("cattle-oidc-client-secrets", fakeClientID).Return(&v1.Secret{
					Data: map[string][]byte{
						fakeClientSecretID: []byte("invalid"),
					},
				}, nil)
			},
			wantError: `{"error":"invalid_request","error_description":"invalid client_secret"}`,
		},
		"authorization_code fails for an invalid code verifier (PKCE)": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "authorization_code")
				data.Set("code", fakeCode)
				data.Set("code_verifier", fakeCodeVerifier)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.sessionClient.EXPECT().Get(fakeCode).Return(&session.Session{
					ClientID:      fakeClientID,
					TokenName:     fakeTokenName,
					Scope:         []string{"openid", "profile"},
					CodeChallenge: "invalid",
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{fakeOIDCClient}, nil)
				m.secretCache.EXPECT().Get("cattle-oidc-client-secrets", fakeClientID).Return(fakeClientk8sSecret, nil)
				m.oidcClient.EXPECT().Patch(fakeClientName, types.JSONPatchType, clientSecretIDPatch).Return(fakeOIDCClient, nil)
			},
			wantError: `{"error":"invalid_request","error_description":"failed to verify PKCE code challenge"}`,
		},
		"authorization_code fails for a disabled token": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "authorization_code")
				data.Set("code", fakeCode)
				data.Set("code_verifier", fakeCodeVerifier)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.sessionClient.EXPECT().Get(fakeCode).Return(fakeSession, nil)
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{fakeOIDCClient}, nil)
				m.secretCache.EXPECT().Get("cattle-oidc-client-secrets", fakeClientID).Return(fakeClientk8sSecret, nil)
				m.oidcClient.EXPECT().Patch(fakeClientName, types.JSONPatchType, clientSecretIDPatch).Return(fakeOIDCClient, nil)
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					UserID:       fakeUserID,
					Enabled:      ptr.To(false),
					AuthProvider: fakeAuthProvider,
				}, nil)

			},
			wantError: `{"error":"access_denied","error_description":"Rancher token is disabled"}`,
		},
		"authorization_code fails for a disabled user": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "authorization_code")
				data.Set("code", fakeCode)
				data.Set("code_verifier", fakeCodeVerifier)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.sessionClient.EXPECT().Get(fakeCode).Return(fakeSession, nil)
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{fakeOIDCClient}, nil)
				m.secretCache.EXPECT().Get("cattle-oidc-client-secrets", fakeClientID).Return(fakeClientk8sSecret, nil)
				m.oidcClient.EXPECT().Patch(fakeClientName, types.JSONPatchType, clientSecretIDPatch).Return(fakeOIDCClient, nil)
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(fakeToken, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					DisplayName: fakeUsername,
					Enabled:     ptr.To(false),
				}, nil)

			},
			wantError: `{"error":"access_denied","error_description":"user is disabled"}`,
		},
		"authorization_code returns a refresh_token when offline_token scope is provided": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "authorization_code")
				data.Set("code", fakeCode)
				data.Set("code_verifier", fakeCodeVerifier)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.sessionClient.EXPECT().Get(fakeCode).Return(fakeSessionOfflineAccess, nil)
				m.sessionClient.EXPECT().Remove(fakeCode).Return(nil)
				m.secretCache.EXPECT().Get("cattle-oidc-client-secrets", fakeClientID).Return(fakeClientk8sSecret, nil)
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{fakeOIDCClient}, nil)
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(fakeToken, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(fakeUser, nil)
				m.useAttributeLister.EXPECT().Get(fakeUserID).Return(fakeUserAttributes, nil)
				m.tokenClient.EXPECT().Patch(fakeTokenName, types.JSONPatchType, tokenPatch).Return(fakeToken, nil)
				m.signingKeyGetter.EXPECT().GetSigningKey().Return(privateKey, fakeSigningKey, nil)
				m.oidcClient.EXPECT().Patch(fakeClientName, types.JSONPatchType, clientSecretIDPatch).Return(fakeOIDCClient, nil)
			},
			wantIdTokenClaims: &jwt.MapClaims{
				"aud":           []any{fakeClientID},
				"exp":           float64(fakeTime().Add(fakeTokenLifespan * time.Second).Unix()),
				"iss":           settings.ServerURL.Get() + "/oidc",
				"iat":           float64(fakeTime().Unix()),
				"name":          fakeUsername,
				"sub":           fakeUserID,
				"auth_provider": fakeAuthProvider,
				"groups":        []any{fakeGroup},
			},
			wantAccessTokenClaims: &jwt.MapClaims{
				"aud":           []any{fakeClientID},
				"exp":           float64(fakeTime().Add(fakeTokenLifespan * time.Second).Unix()),
				"iss":           settings.ServerURL.Get() + "/oidc",
				"iat":           float64(fakeTime().Unix()),
				"sub":           fakeUserID,
				"auth_provider": fakeAuthProvider,
				"scope":         fakeScopesOfflineAccess,
				"token":         fakeToken.Name + ":" + fakeToken.Token,
			},
			wantRefreshTokenClaims: &jwt.MapClaims{
				"aud":                []any{fakeClientID},
				"exp":                float64(fakeTime().Add(fakeRefreshTokenLifespan * time.Second).Unix()),
				"iat":                float64(fakeTime().Unix()),
				"sub":                fakeUserID,
				"auth_provider":      fakeAuthProvider,
				"scope":              fakeScopesOfflineAccess,
				"rancher_token_hash": rancherTokenHash,
			},
		},
		"refresh_token returns new refresh token": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "refresh_token")
				data.Set("refresh_token", fakeRefreshTokenString)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{fakeOIDCClient}, nil)
				m.tokenCache.EXPECT().List(labels.SelectorFromSet(map[string]string{
					tokens.UserIDLabel: fakeUserID,
				})).Return(fakeTokenList, nil)
				m.tokenClient.EXPECT().Patch(fakeTokenName, types.JSONPatchType, tokenPatch).Return(fakeToken, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(fakeUser, nil)
				m.useAttributeLister.EXPECT().Get(fakeUserID).Return(fakeUserAttributes, nil)
				m.signingKeyGetter.EXPECT().GetSigningKey().Return(privateKey, fakeSigningKey, nil)
				m.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
			},
			wantIdTokenClaims: &jwt.MapClaims{
				"aud":           []any{fakeClientID},
				"exp":           float64(fakeTime().Add(fakeTokenLifespan * time.Second).Unix()),
				"iss":           settings.ServerURL.Get() + "/oidc",
				"iat":           float64(fakeTime().Unix()),
				"name":          fakeUsername,
				"sub":           fakeUserID,
				"auth_provider": fakeAuthProvider,
				"groups":        []any{fakeGroup},
			},
			wantAccessTokenClaims: &jwt.MapClaims{
				"aud":           []any{fakeClientID},
				"exp":           float64(fakeTime().Add(fakeTokenLifespan * time.Second).Unix()),
				"iss":           settings.ServerURL.Get() + "/oidc",
				"iat":           float64(fakeTime().Unix()),
				"sub":           fakeUserID,
				"auth_provider": fakeAuthProvider,
				"scope":         fakeScopesOfflineAccess,
				"token":         fakeToken.Name + ":" + fakeToken.Token,
			},
			wantRefreshTokenClaims: &jwt.MapClaims{
				"aud":                []any{fakeClientID},
				"exp":                float64(fakeTime().Add(fakeRefreshTokenLifespan * time.Second).Unix()),
				"iat":                float64(fakeTime().Unix()),
				"sub":                fakeUserID,
				"auth_provider":      fakeAuthProvider,
				"scope":              fakeScopesOfflineAccess,
				"rancher_token_hash": rancherTokenHash,
			},
		},
		"refresh_token fails to validate signature": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "refresh_token")
				data.Set("refresh_token", fakeRefreshTokenString)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				anotherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				m.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&anotherKey.PublicKey, nil)
			},
			wantError: `{"error":"server_error","error_description":"failed to parse refresh token: token signature is invalid: crypto/rsa: verification error"}`,
		},
		"refresh_token fails when the associated Rancher token is no longer present": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "refresh_token")
				data.Set("refresh_token", fakeRefreshTokenString)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				m.tokenCache.EXPECT().List(labels.SelectorFromSet(map[string]string{
					tokens.UserIDLabel: fakeUserID,
				})).Return([]*v3.Token{}, nil)

			},
			wantError: `{"error":"access_denied","error_description":"Rancher token no longer present."}`,
		},
		"refresh_token fails when the associated Rancher token has expired": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "refresh_token")
				data.Set("refresh_token", fakeRefreshTokenString)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{fakeOIDCClient}, nil)
				m.tokenCache.EXPECT().List(labels.SelectorFromSet(map[string]string{
					tokens.UserIDLabel: fakeUserID,
				})).Return(fakeTokenExpiredList, nil)
				m.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
			},
			wantError: `{"error":"access_denied","error_description":"Rancher token has expired"}`,
		},
		"refresh_token fails when the OIDC client doesn't exist": {
			req: func() *http.Request {
				data := url.Values{}
				data.Set("grant_type", "refresh_token")
				data.Set("refresh_token", fakeRefreshTokenString)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
				m.tokenCache.EXPECT().List(labels.SelectorFromSet(map[string]string{
					tokens.UserIDLabel: fakeUserID,
				})).Return(fakeTokenList, nil)
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{}, nil)

			},
			wantError: `{"error":"server_error","error_description":"failed to get oidc client: no OIDC clients found"}`,
		},
		"refresh_token fails when it is expired": {
			req: func() *http.Request {
				refreshTokenExpired := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
					"aud":                []string{fakeClientID},
					"exp":                time.Unix(0, 0).Unix(),
					"iat":                now.Unix(),
					"sub":                fakeUserID,
					"rancher_token_hash": rancherTokenHash,
					"scope":              fakeScopesOfflineAccess,
				})
				refreshTokenExpired.Header["kid"] = fakeSigningKey
				refreshTokenExpiredString, _ := refreshTokenExpired.SignedString(privateKey)

				data := url.Values{}
				data.Set("grant_type", "refresh_token")
				data.Set("refresh_token", refreshTokenExpiredString)
				req, _ := http.NewRequest("POST", "https://rancher.com", bytes.NewBufferString(data.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fakeClientID+":"+fakeClientSecret))))

				return req
			},
			mockSetup: func(m mockParams) {
				m.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
			},
			wantError: `{"error":"server_error","error_description":"failed to parse refresh token: token has invalid claims: token is expired"}`,
		},
	}

	// register auth provider
	mockProvider := providermocks.NewMockAuthProvider(ctrl)
	mockProvider.EXPECT().IsDisabledProvider().Return(false, nil).AnyTimes()
	providers.Providers[fakeAuthProvider] = mockProvider

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := mockParams{
				tokenCache:         fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl),
				tokenClient:        fake.NewMockNonNamespacedClientInterface[*v3.Token, *v3.TokenList](ctrl),
				secretCache:        fake.NewMockCacheInterface[*v1.Secret](ctrl),
				userLister:         fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl),
				useAttributeLister: fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl),
				oidcClientCache:    fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctrl),
				oidcClient:         fake.NewMockNonNamespacedClientInterface[*v3.OIDCClient, *v3.OIDCClientList](ctrl),
				sessionClient:      mocks.NewMocksessionGetterRemover(ctrl),
				signingKeyGetter:   mocks.NewMocksigningKeyGetter(ctrl),
			}
			if test.mockSetup != nil {
				test.mockSetup(m)
			}
			h := newTokenHandler(m.tokenCache, m.userLister, m.useAttributeLister, m.sessionClient, m.signingKeyGetter, m.oidcClientCache, m.oidcClient, m.secretCache, m.tokenClient)
			h.now = fakeTime
			rec := httptest.NewRecorder()

			h.tokenEndpoint(rec, test.req())

			if test.wantError != "" {
				assert.JSONEq(t, test.wantError, strings.TrimSpace(rec.Body.String()))
			} else {
				var tokenResponse TokenResponse
				err := json.Unmarshal(rec.Body.Bytes(), &tokenResponse)
				assert.NoError(t, err)
				assert.Equal(t, tokenResponse.TokenType, bearerTokenType)
				if test.wantIdTokenClaims != nil {
					claims := jwt.MapClaims{}
					_, err := jwt.ParseWithClaims(tokenResponse.IDToken, &claims, func(token *jwt.Token) (any, error) {
						return &privateKey.PublicKey, nil
					})
					assert.NoError(t, err)
					assert.Equal(t, test.wantIdTokenClaims, &claims, "id token does not match")
				} else {
					assert.Empty(t, tokenResponse.IDToken)
				}
				if test.wantAccessTokenClaims != nil {
					claims := jwt.MapClaims{}
					_, err := jwt.ParseWithClaims(tokenResponse.AccessToken, &claims, func(token *jwt.Token) (any, error) {
						return &privateKey.PublicKey, nil
					})
					assert.NoError(t, err)
					assert.Equal(t, test.wantAccessTokenClaims, &claims, "access token does not match")
				} else {
					assert.Empty(t, tokenResponse.AccessToken)
				}
				if test.wantRefreshTokenClaims != nil {
					claims := jwt.MapClaims{}
					_, err := jwt.ParseWithClaims(tokenResponse.RefreshToken, &claims, func(token *jwt.Token) (any, error) {
						return &privateKey.PublicKey, nil
					})
					assert.NoError(t, err)
					assert.Equal(t, test.wantRefreshTokenClaims, &claims, "refresh token does not match")
				} else {
					assert.Empty(t, tokenResponse.RefreshToken)
				}
			}
		})
	}
}
