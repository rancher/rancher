package oidc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/oauth2"
)

func TestValidateACR(t *testing.T) {
	tests := []struct {
		name          string
		claimACR      string
		configuredACR string
		want          bool
	}{
		{
			name:          "acr set in config and matches token",
			configuredACR: "level1",
			claimACR:      "level1",
			want:          true,
		},
		{
			name:          "acr set in config and does not match token",
			configuredACR: "level1",
			claimACR:      "",
			want:          false,
		},
		{
			name:     "acr not set in config",
			claimACR: "",
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isValidACR(tt.claimACR, tt.configuredACR), "isValidACR(%v, %v)", tt.claimACR, tt.configuredACR)
		})
	}
}

func TestParseACRFromAccessToken(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	suffix := base64.URLEncoding.EncodeToString([]byte(`{}`))
	validClaims := base64.RawURLEncoding.EncodeToString([]byte(`{"acr":"example_acr"}`))
	invalidBase64Claims := "invalid_base64_claims"
	noAcrClaims := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"1234567890"}`))

	tests := []struct {
		name        string
		token       string
		expectedACR string
		wantError   bool
	}{
		{
			name:        "valid token with ACR",
			token:       fmt.Sprintf("%s.%s.%s", header, validClaims, suffix),
			expectedACR: "example_acr",
		},
		{
			name:        "invalid token format",
			token:       "invalid.token",
			expectedACR: "",
			wantError:   true,
		},
		{
			name:        "invalid base64 decoding",
			token:       fmt.Sprintf("%s.%s.", header, invalidBase64Claims),
			expectedACR: "",
			wantError:   true,
		},
		{
			name:        "valid token without ACR claim",
			token:       fmt.Sprintf("%s.%s.suffix", header, noAcrClaims),
			expectedACR: "",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acr, err := parseACRFromAccessToken(tt.token)
			if acr != tt.expectedACR {
				t.Errorf("expected acr to be '%s', got '%s'", tt.expectedACR, acr)
			}
			if (err != nil) != tt.wantError {
				t.Errorf("expected error: %v, got error: %v", tt.wantError, err)
			}
		})
	}
}

func TestGetUserInfoFromAuthCode(t *testing.T) {
	const (
		providerName = "keycloak"
		userId       = "user"
	)
	ctrl := gomock.NewController(t)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		assert.NoError(t, err)
	}
	tests := map[string]struct {
		config                    func(string) *apiv3.OIDCConfig
		authCode                  string
		tokenManagerMock          func(token *Token) tokenManager
		oidcProviderResponses     func(string) oidcResponses
		expectedUserInfoSubject   string
		expectedUserInfoClaimInfo ClaimInfo
		expectedClaimInfo         *ClaimInfo
		expectedErrorMessage      string
	}{
		"token is updated and userInfo returned": {
			config: func(port string) *apiv3.OIDCConfig {
				return newOIDCConfig(port)
			},
			tokenManagerMock: func(token *Token) tokenManager {
				mock := mocks.NewMocktokenManager(ctrl)
				mock.EXPECT().UpdateSecret(userId, providerName, EqToken(token.IDToken))

				return mock
			},
			oidcProviderResponses: func(port string) oidcResponses {
				return newOIDCResponses(privateKey, port)
			},
			expectedUserInfoSubject: "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
			expectedUserInfoClaimInfo: ClaimInfo{
				Subject:       "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
				Groups:        []string{"admingroup"},
				FullGroupPath: []string{"/admingroup"},
				Roles:         []string{"adminrole"},
			},
			expectedClaimInfo: &ClaimInfo{},
		},
		"get groups with GroupsClaims": {
			config: func(port string) *apiv3.OIDCConfig {
				c := newOIDCConfig(port)
				c.GroupsClaim = "custom:groups"

				return c
			},
			oidcProviderResponses: func(port string) oidcResponses {
				res := newOIDCResponses(privateKey, port)
				tokenJWT := jwt.New(jwt.SigningMethodRS256)
				tokenJWT.Claims = jwt.MapClaims{
					"aud":           "test",
					"exp":           time.Now().Add(5 * time.Minute).Unix(), // expires in the future
					"iss":           "http://localhost:" + port,
					"custom:groups": []string{"group1", "group2"},
				}
				tokenStr, err := tokenJWT.SignedString(privateKey)
				assert.NoError(t, err)

				token := &Token{
					Token: oauth2.Token{
						AccessToken:  tokenStr,
						Expiry:       time.Now().Add(5 * time.Minute), // expires in the future
						RefreshToken: tokenStr,
					},
					IDToken: tokenStr,
				}
				res.token = token
				res.user = `{
				"sub": "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd"
              }`
				return res
			},
			tokenManagerMock: func(token *Token) tokenManager {
				mock := mocks.NewMocktokenManager(ctrl)
				mock.EXPECT().UpdateSecret(userId, providerName, EqToken(token.IDToken))

				return mock
			},
			expectedUserInfoSubject: "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
			expectedUserInfoClaimInfo: ClaimInfo{
				Subject: "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
			},
			expectedClaimInfo: &ClaimInfo{
				Groups: []string{"group1", "group2"},
			},
		},
		"error - invalid certificate": {
			config: func(port string) *apiv3.OIDCConfig {
				return &apiv3.OIDCConfig{
					Issuer:      "http://localhost:" + port,
					ClientID:    "test",
					JWKSUrl:     "http://localhost:" + port + "/.well-known/jwks.json",
					Certificate: "invalid",
					PrivateKey:  "invalid",
				}
			},
			tokenManagerMock: func(token *Token) tokenManager {
				return mocks.NewMocktokenManager(ctrl)
			},
			oidcProviderResponses: func(port string) oidcResponses {
				return newOIDCResponses(privateKey, port)
			},
			expectedErrorMessage: "could not parse cert/key pair: tls: failed to find any PEM data in certificate input",
		},
		"error - invalid token from server": {
			config: func(port string) *apiv3.OIDCConfig {
				return newOIDCConfig(port)
			},
			tokenManagerMock: func(token *Token) tokenManager {
				return mocks.NewMocktokenManager(ctrl)
			},
			oidcProviderResponses: func(port string) oidcResponses {
				resp := newOIDCResponses(privateKey, port)
				resp.token.IDToken = "invalid"

				return resp
			},
			expectedErrorMessage: "oidc: malformed jwt",
		},
		"error - invalid user response": {
			config: func(port string) *apiv3.OIDCConfig {
				return newOIDCConfig(port)
			},
			tokenManagerMock: func(token *Token) tokenManager {
				mock := mocks.NewMocktokenManager(ctrl)
				mock.EXPECT().UpdateSecret(userId, providerName, EqToken(token.IDToken))

				return mock
			},
			oidcProviderResponses: func(port string) oidcResponses {
				resp := newOIDCResponses(privateKey, port)
				resp.user = "invalid"

				return resp
			},
			expectedErrorMessage: "oidc: failed to decode userinfo",
		},
		"display name with custom nameClaim": {
			config: func(port string) *apiv3.OIDCConfig {
				c := newOIDCConfig(port)
				c.NameClaim = "display_name"

				return c
			},
			oidcProviderResponses: func(port string) oidcResponses {
				res := newOIDCResponses(privateKey, port)
				tokenJWT := jwt.New(jwt.SigningMethodRS256)
				tokenJWT.Claims = jwt.MapClaims{
					"name":         "test_user",
					"aud":          "test",
					"exp":          time.Now().Add(5 * time.Minute).Unix(), // expires in the future
					"email":        "test@example.com",
					"iss":          "http://localhost:" + port,
					"display_name": "Test User",
				}
				tokenStr, err := tokenJWT.SignedString(privateKey)
				assert.NoError(t, err)

				token := &Token{
					Token: oauth2.Token{
						AccessToken:  tokenStr,
						Expiry:       time.Now().Add(5 * time.Minute), // expires in the future
						RefreshToken: tokenStr,
					},
					IDToken: tokenStr,
				}
				res.token = token
				res.user = `{
				"sub": "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd"
              }`
				return res
			},
			tokenManagerMock: func(token *Token) tokenManager {
				mock := mocks.NewMocktokenManager(ctrl)
				mock.EXPECT().UpdateSecret(userId, providerName, EqToken(token.IDToken))

				return mock
			},
			expectedUserInfoSubject: "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
			expectedUserInfoClaimInfo: ClaimInfo{
				Subject: "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
			},
			expectedClaimInfo: &ClaimInfo{
				Name:  "Test User",
				Email: "test@example.com",
			},
		},
		"display name with custom emailClaim": {
			config: func(port string) *apiv3.OIDCConfig {
				c := newOIDCConfig(port)
				c.EmailClaim = "public_email"

				return c
			},
			oidcProviderResponses: func(port string) oidcResponses {
				res := newOIDCResponses(privateKey, port)
				tokenJWT := jwt.New(jwt.SigningMethodRS256)
				tokenJWT.Claims = jwt.MapClaims{
					"aud":          "test",
					"exp":          time.Now().Add(5 * time.Minute).Unix(), // expires in the future
					"email":        "test@dev.example.com",
					"iss":          "http://localhost:" + port,
					"public_email": "test.dev@example.com",
				}
				tokenStr, err := tokenJWT.SignedString(privateKey)
				assert.NoError(t, err)

				token := &Token{
					Token: oauth2.Token{
						AccessToken:  tokenStr,
						Expiry:       time.Now().Add(5 * time.Minute), // expires in the future
						RefreshToken: tokenStr,
					},
					IDToken: tokenStr,
				}
				res.token = token
				res.user = `{
				"sub": "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd"
              }`
				return res
			},
			tokenManagerMock: func(token *Token) tokenManager {
				mock := mocks.NewMocktokenManager(ctrl)
				mock.EXPECT().UpdateSecret(userId, providerName, EqToken(token.IDToken))

				return mock
			},
			expectedUserInfoSubject: "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
			expectedUserInfoClaimInfo: ClaimInfo{
				Subject: "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
			},
			expectedClaimInfo: &ClaimInfo{
				Email: "test.dev@example.com",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			listener, err := net.Listen("tcp", ":0") // choose any available port
			assert.NoError(t, err)
			port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
			oidcResp := test.oidcProviderResponses(port)
			server := mockOIDCServer(listener, oidcResp)
			defer server.Shutdown(context.TODO())
			o := OpenIDCProvider{
				Name:     providerName,
				TokenMgr: test.tokenManagerMock(oidcResp.token),
			}
			claimInfo := &ClaimInfo{}

			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "https://localhost:"+port, nil)
			userInfo, token, idToken, err := o.getUserInfoFromAuthCode(rw, req, test.config(port), test.authCode, claimInfo, userId)
			if test.expectedErrorMessage != "" {
				assert.ErrorContains(t, err, test.expectedErrorMessage)
			} else {
				assert.NoError(t, err)
				claims := ClaimInfo{}
				assert.Equal(t, test.expectedClaimInfo, claimInfo)
				assert.NoError(t, userInfo.Claims(&claims))
				assert.Equal(t, test.expectedUserInfoSubject, userInfo.Subject)
				assert.Equal(t, test.expectedUserInfoClaimInfo, claims)
				assert.Equal(t, oidcResp.token.AccessToken, token.AccessToken) //token should be the same as the one returned by the mock oidc server.
				assert.NotEmpty(t, idToken)                                    // the token is generated each time.
			}
		})
	}
}

func TestGetClaimInfoFromToken(t *testing.T) {
	const (
		providerName = "keycloak"
		userId       = "user"
	)

	ctrl := gomock.NewController(t)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		assert.NoError(t, err)
	}

	tests := map[string]struct {
		config                func(string) *apiv3.OIDCConfig
		storedToken           func(string) *oauth2.Token
		tokenManagerMock      func(token *Token) tokenManager
		oidcProviderResponses func(string) oidcResponses
		expectedClaimInfo     *ClaimInfo
		expectedErrorMessage  string
	}{
		"get claims with valid token": {
			config: func(port string) *apiv3.OIDCConfig {
				return newOIDCConfig(port)
			},
			storedToken: func(port string) *oauth2.Token {
				token := jwt.New(jwt.SigningMethodRS256)
				token.Claims = jwt.RegisteredClaims{
					Audience: []string{"test"},
					// expires in the future
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
					Issuer:    "http://localhost:" + port,
				}
				tokenStr, err := token.SignedString(privateKey)
				assert.NoError(t, err)

				return &oauth2.Token{
					AccessToken: tokenStr,
					Expiry:      time.Now().Add(5 * time.Minute),
				}
			},
			oidcProviderResponses: func(port string) oidcResponses {
				return newOIDCResponses(privateKey, port)
			},
			tokenManagerMock: func(_ *Token) tokenManager {
				return mocks.NewMocktokenManager(ctrl)
			},
			expectedClaimInfo: &ClaimInfo{
				Subject:       "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
				Groups:        []string{"admingroup"},
				FullGroupPath: []string{"/admingroup"},
				Roles:         []string{"adminrole"},
			},
		},
		"token is refreshed and updated when expired": {
			config: func(port string) *apiv3.OIDCConfig {
				return newOIDCConfig(port)
			},
			oidcProviderResponses: func(port string) oidcResponses {
				return newOIDCResponses(privateKey, port)
			},
			storedToken: func(port string) *oauth2.Token {
				token := jwt.New(jwt.SigningMethodRS256)
				token.Claims = jwt.RegisteredClaims{
					Audience:  []string{"test"},
					ExpiresAt: jwt.NewNumericDate(time.Unix(0, 0)), // has expired
					Issuer:    "http://localhost:" + port,
				}
				tokenStr, err := token.SignedString(privateKey)
				assert.NoError(t, err)
				refreshToken := jwt.New(jwt.SigningMethodRS256)
				refreshToken.Claims = jwt.RegisteredClaims{
					Audience: []string{"test"},
					// expires in the future
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 *
						time.Minute)),
					Issuer: "http://localhost:" + port,
				}
				refreshTokenStr, err := refreshToken.SignedString(privateKey)
				assert.NoError(t, err)

				return &oauth2.Token{
					AccessToken:  tokenStr,
					Expiry:       time.Unix(0, 0), // has expired
					RefreshToken: refreshTokenStr,
				}
			},
			expectedClaimInfo: &ClaimInfo{
				Subject:       "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
				Groups:        []string{"admingroup"},
				FullGroupPath: []string{"/admingroup"},
				Roles:         []string{"adminrole"},
			},
			tokenManagerMock: func(token *Token) tokenManager {
				mock := mocks.NewMocktokenManager(ctrl)
				mock.EXPECT().UpdateSecret(userId, providerName, EqToken(token.RefreshToken))

				return mock
			},
		},
		"error - invalid certificate": {
			config: func(port string) *apiv3.OIDCConfig {
				return &apiv3.OIDCConfig{
					Issuer:      "http://localhost:" + port,
					ClientID:    "test",
					JWKSUrl:     "http://localhost:" + port + "/.well-known/jwks.json",
					Certificate: "invalid",
					PrivateKey:  "invalid",
				}
			},
			storedToken: func(port string) *oauth2.Token {
				return &oauth2.Token{}
			},
			oidcProviderResponses: func(port string) oidcResponses {
				return newOIDCResponses(privateKey, port)
			},
			tokenManagerMock: func(_ *Token) tokenManager {
				return mocks.NewMocktokenManager(ctrl)
			},
			expectedClaimInfo:    nil,
			expectedErrorMessage: "could not parse cert/key pair: tls: failed to find any PEM data in certificate input",
		},
		"error - invalid token": {
			config: func(port string) *apiv3.OIDCConfig {
				return newOIDCConfig(port)
			},
			storedToken: func(port string) *oauth2.Token {
				return &oauth2.Token{
					AccessToken: "invalid",
				}
			},
			oidcProviderResponses: func(port string) oidcResponses {
				return newOIDCResponses(privateKey, port)
			},
			tokenManagerMock: func(_ *Token) tokenManager {
				return mocks.NewMocktokenManager(ctrl)
			},
			expectedClaimInfo:    nil,
			expectedErrorMessage: "oidc: malformed jwt",
		},
		"error - invalid user response": {
			config: func(port string) *apiv3.OIDCConfig {
				return newOIDCConfig(port)
			},
			storedToken: func(port string) *oauth2.Token {
				token := jwt.New(jwt.SigningMethodRS256)
				token.Claims = jwt.RegisteredClaims{
					Audience: []string{"test"},
					// expires in the future
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 *
						time.Minute)),
					Issuer: "http://localhost:" + port,
				}
				tokenStr, err := token.SignedString(privateKey)
				assert.NoError(t, err)

				return &oauth2.Token{
					AccessToken: tokenStr,
					Expiry:      time.Now().Add(5 * time.Minute), // expires in the future
				}
			},
			oidcProviderResponses: func(port string) oidcResponses {
				resp := newOIDCResponses(privateKey, port)
				resp.user = "invalid"

				return resp
			},
			tokenManagerMock: func(_ *Token) tokenManager {
				return mocks.NewMocktokenManager(ctrl)
			},
			expectedClaimInfo:    nil,
			expectedErrorMessage: "oidc: failed to decode userinfo",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			listener, err := net.Listen("tcp", ":0") // choose any available port
			assert.NoError(t, err)
			port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
			oidcResp := test.oidcProviderResponses(port)
			server := mockOIDCServer(listener, oidcResp)
			assert.NoError(t, err)
			defer server.Shutdown(context.TODO())
			o := OpenIDCProvider{
				Name:     providerName,
				TokenMgr: test.tokenManagerMock(oidcResp.token),
			}

			claimsInfo, err := o.getClaimInfoFromToken(context.TODO(), test.config(port), test.storedToken(port), userId)

			assert.Equal(t, test.expectedClaimInfo, claimsInfo)
			if test.expectedErrorMessage == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedErrorMessage)
			}
		})
	}
}

func TestGetGroupsFromClaimInfo(t *testing.T) {
	type args struct {
		claimInfo ClaimInfo
	}
	type want struct {
		groupNames []string
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "get groups from claim info",
			args: args{
				claimInfo: ClaimInfo{
					Groups: []string{"group1", "group2"},
				},
			},
			want: want{
				groupNames: []string{"group1", "group2"},
			},
		},
		{
			name: "roles and groups are combined",
			args: args{
				claimInfo: ClaimInfo{
					Groups: []string{"group1", "group2"},
					Roles:  []string{"role1", "role2"},
				},
			},
			want: want{
				groupNames: []string{"group1", "group2", "role1", "role2"},
			},
		},
		{
			name: "just roles",
			args: args{
				claimInfo: ClaimInfo{
					Roles: []string{"role1", "role2"},
				},
			},
			want: want{
				groupNames: []string{"role1", "role2"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OpenIDCProvider{
				Name: "oidc",
			}
			got := o.getGroupsFromClaimInfo(tt.args.claimInfo)
			var gotGroupNames []string
			for _, principal := range got {
				parts := strings.Split(principal.Name, "://")
				if len(parts) == 2 {
					gotGroupNames = append(gotGroupNames, parts[1])
				}
			}
			sort.Strings(gotGroupNames)
			assert.Equal(t, tt.want.groupNames, gotGroupNames)
		})
	}
}

const (
	logoutPath    = "/v3/tokens?action=logout"
	logoutAllPath = "/v3/tokens?action=logoutAll"
)

func TestLogout(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "keycloak"
	)

	logoutTests := map[string]struct {
		config *apiv3.OIDCConfig
		verify func(t require.TestingT, err error, msgAndArgs ...any)
	}{
		"when logout all is forced": {
			config: newOIDCConfig("9090", func(s *apiv3.OIDCConfig) {
				s.LogoutAllForced = true
			}),
			verify: require.Error,
		},
		"when logout all is not forced": {
			config: newOIDCConfig("9090"),
			verify: require.NoError,
		},
	}

	for name, tt := range logoutTests {
		t.Run(name, func(t *testing.T) {
			testToken := &apiv3.Token{UserID: userId, AuthProvider: providerName}
			o := OpenIDCProvider{
				Name:      providerName,
				GetConfig: func() (*apiv3.OIDCConfig, error) { return tt.config, nil },
			}
			b, err := json.Marshal(&apiv3.AuthConfigLogoutInput{
				FinalRedirectURL: "https://example.com/logged-out",
			})
			require.NoError(t, err)

			r := httptest.NewRequest(http.MethodPost, logoutPath, bytes.NewReader(b))
			r.AddCookie(&http.Cookie{Name: "R_OIDC_ID", Value: "test-id-token"})
			w := httptest.NewRecorder()

			tt.verify(t, o.Logout(w, r, testToken))
		})
	}
}

func TestLogoutAllWhenNotEnabled(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "keycloak"
	)

	oidcConfig := newOIDCConfig("8090", func(s *apiv3.OIDCConfig) {
		s.EndSessionEndpoint = "http://localhost:8090/user/logout"
		s.LogoutAllEnabled = false
	})
	testToken := &apiv3.Token{UserID: userId, AuthProvider: providerName}
	o := OpenIDCProvider{
		Name:      providerName,
		GetConfig: func() (*apiv3.OIDCConfig, error) { return oidcConfig, nil },
	}
	b, err := json.Marshal(&apiv3.AuthConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, logoutPath, bytes.NewReader(b))
	w := httptest.NewRecorder()

	assert.ErrorContains(t, o.LogoutAll(w, r, testToken), "Rancher provider resource `keycloak` not configured for SLO")
}

func TestLogoutAll(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "keycloak"
	)

	oidcConfig := newOIDCConfig("8090", func(s *apiv3.OIDCConfig) {
		s.EndSessionEndpoint = "http://localhost:8090/user/logout"
	})
	testToken := &apiv3.Token{UserID: userId, AuthProvider: providerName}
	o := OpenIDCProvider{
		Name:      providerName,
		GetConfig: func() (*apiv3.OIDCConfig, error) { return oidcConfig, nil },
	}
	b, err := json.Marshal(&apiv3.AuthConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, logoutAllPath, bytes.NewReader(b))
	w := httptest.NewRecorder()

	require.NoError(t, o.LogoutAll(w, r, testToken))

	require.Equal(t, http.StatusOK, w.Code)
	wantData := map[string]any{
		"idpRedirectUrl": "http://localhost:8090/user/logout?client_id=test&post_logout_redirect_uri=https%3A%2F%2Fexample.com%2Flogged-out",
		"type":           "authConfigLogoutOutput",
		"baseType":       "authConfigLogoutOutput",
	}
	gotData := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gotData))
	assert.Equal(t, wantData, gotData)
}

func TestLogoutAllNoEndSessionEndpoint(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "oidc"
	)

	oidcConfig := newOIDCConfig("8090")
	testToken := &apiv3.Token{UserID: userId, AuthProvider: providerName}
	o := OpenIDCProvider{
		Name:      providerName,
		GetConfig: func() (*apiv3.OIDCConfig, error) { return oidcConfig, nil },
	}
	b, err := json.Marshal(&apiv3.AuthConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, "/v3/tokens?action=logoutAll", bytes.NewReader(b))
	w := httptest.NewRecorder()

	assert.ErrorContains(t, o.LogoutAll(w, r, testToken), "LogoutAll triggered with no endSessionEndpoint")
}

func TestLogoutWithIDToken(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "oidc"
	)
	oidcConfig := newOIDCConfig("8090", func(s *apiv3.OIDCConfig) {
		s.EndSessionEndpoint = "http://localhost:8090/user/logout"
	})

	testToken := &apiv3.Token{UserID: userId, AuthProvider: providerName}
	o := OpenIDCProvider{
		Name:      providerName,
		GetConfig: func() (*apiv3.OIDCConfig, error) { return oidcConfig, nil },
	}

	b, err := json.Marshal(&apiv3.AuthConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader(b))
	req.AddCookie(&http.Cookie{Name: "R_OIDC_ID", Value: "test-id-token"})
	w := httptest.NewRecorder()

	require.NoError(t, o.LogoutAll(w, req, testToken))
	wantData := map[string]any{
		"baseType":       "authConfigLogoutOutput",
		"idpRedirectUrl": "http://localhost:8090/user/logout?client_id=test&id_token_hint=test-id-token&post_logout_redirect_uri=https%3A%2F%2Fexample.com%2Flogged-out",
		"type":           "authConfigLogoutOutput",
	}
	gotData := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gotData))
	assert.Equal(t, wantData, gotData)
}

func TestLogoutAllNoIDToken(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "oidc"
	)
	oidcConfig := newOIDCConfig("8090", func(s *apiv3.OIDCConfig) {
		s.EndSessionEndpoint = "http://localhost:8090/user/logout"
	})

	testToken := &apiv3.Token{UserID: userId, AuthProvider: providerName}
	o := OpenIDCProvider{
		Name:      providerName,
		GetConfig: func() (*apiv3.OIDCConfig, error) { return oidcConfig, nil },
	}

	b, err := json.Marshal(&apiv3.AuthConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader(b))
	w := httptest.NewRecorder()

	require.NoError(t, o.LogoutAll(w, req, testToken))
	wantData := map[string]any{
		"baseType":       "authConfigLogoutOutput",
		"idpRedirectUrl": "http://localhost:8090/user/logout?client_id=test&post_logout_redirect_uri=https%3A%2F%2Fexample.com%2Flogged-out",
		"type":           "authConfigLogoutOutput",
	}
	gotData := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gotData))
	assert.Equal(t, wantData, gotData)
}

// mockOIDCServer creates an http server that mocks an OIDC provider. Responses are passed as a parameter.
func mockOIDCServer(listener net.Listener, resp oidcResponses) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.config)
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.jwks)
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp.user))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.token)
	})

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		_ = server.Serve(listener)
	}()

	return server
}

type oidcResponses struct {
	user   string
	config providerJSON
	jwks   jsonWebKeySet
	token  *Token
}

type Token struct {
	oauth2.Token
	IDToken string `json:"id_token"`
}

func newOIDCResponses(privateKey *rsa.PrivateKey, port string) oidcResponses {
	jwtToken := jwt.New(jwt.SigningMethodRS256)
	jwtToken.Claims = jwt.RegisteredClaims{
		Audience: []string{"test"},
		// has expired
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		Issuer:    "http://localhost:" + port,
	}
	jwtSrt, _ := jwtToken.SignedString(privateKey)
	// token returned from the /token endpoint
	token := &Token{
		Token: oauth2.Token{
			AccessToken:  jwtSrt,
			Expiry:       time.Now().Add(5 * time.Minute), // expires in the future
			RefreshToken: jwtSrt,
		},
		IDToken: jwtSrt,
	}

	return oidcResponses{
		user: `{
				"sub": "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
				"email_verified": true,
				"groups": [
					"admingroup"
				],
				"full_group_path": [
					"/admingroup"
				],
				"roles": [
					"adminrole"
				]
      }`,
		config: providerJSON{
			Issuer:      "http://localhost:" + port,
			UserInfoURL: "http://localhost:" + port + "/user",
			JWKSURL:     "http://localhost:" + port + "/.well-known/jwks.json",
			AuthURL:     "http://localhost:" + port + "/auth",
			TokenURL:    "http://localhost:" + port + "/token",
		},
		token: token,
		jwks: jsonWebKeySet{
			Keys: []jsonWebKey{
				{
					Kty: "RSA",
					Kid: "example-key-id",
					Use: "sig",
					Alg: "RS256",
					N:   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
					E:   base64.RawURLEncoding.EncodeToString(bigIntToBytes(privateKey.PublicKey.E)),
				},
			},
		},
	}
}

func newOIDCConfig(port string, opts ...func(*apiv3.OIDCConfig)) *apiv3.OIDCConfig {
	cfg := &apiv3.OIDCConfig{
		Issuer:           "http://localhost:" + port,
		ClientID:         "test",
		JWKSUrl:          "http://localhost:" + port + "/.well-known/jwks.json",
		AuthEndpoint:     "http://localhost:" + port + "/auth",
		TokenEndpoint:    "http://localhost:" + port + "/token",
		UserInfoEndpoint: "http://localhost:" + port + "/user",
		LogoutAllEnabled: true,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

type jsonWebKeySet struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// Helper function to convert a big.Int (exponent) to []byte
func bigIntToBytes(i int) []byte {
	var b [4]byte
	b[0] = byte(i >> 24)
	b[1] = byte(i >> 16)
	b[2] = byte(i >> 8)
	b[3] = byte(i)
	return b[:]
}

type providerJSON struct {
	Issuer        string   `json:"issuer"`
	AuthURL       string   `json:"authorization_endpoint"`
	TokenURL      string   `json:"token_endpoint"`
	DeviceAuthURL string   `json:"device_authorization_endpoint"`
	JWKSURL       string   `json:"jwks_uri"`
	UserInfoURL   string   `json:"userinfo_endpoint"`
	Algorithms    []string `json:"id_token_signing_alg_values_supported"`
}

func TestTransformToAuthProvider(t *testing.T) {
	tests := map[string]struct {
		beforeAuthConfig map[string]any
		want             map[string]any
	}{
		"with no API Host": {
			beforeAuthConfig: map[string]any{
				"metadata": map[string]any{
					"name": "genericoidc",
				},
				"clientId":     "rancher",
				"rancherUrl":   "https://localhost:9443/verify-auth",
				"authEndpoint": "https://example.com/realms/rancher/protocol/openid-connect/auth",
			},
			want: map[string]any{
				"id":                 "genericoidc",
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
				"logoutAllSupported": false,
				"redirectUrl":        "https://example.com/realms/rancher/protocol/openid-connect/auth?client_id=rancher&response_type=code&redirect_uri=https://localhost:9443/verify-auth",
			},
		},
		"with an API Host configured": {
			beforeAuthConfig: map[string]any{
				"metadata": map[string]any{
					"name": "genericoidc",
				},
				"rancherApiHost": "https://example.com",
				"clientId":       "rancher",
				"rancherUrl":     "https://localhost:9443/verify-auth",
				"authEndpoint":   "https://example.com/realms/rancher/protocol/openid-connect/auth",
			},
			want: map[string]any{
				"id":                 "genericoidc",
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
				"logoutAllSupported": false,
				"redirectUrl":        "https://example.com/v1-oidc/genericoidc",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := OpenIDCProvider{}
			transformed, err := p.TransformToAuthProvider(tt.beforeAuthConfig)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, transformed)
		})
	}
}

// expiryIn is calculated inside the oauth2 library using time.Now, so we just compare the token is equal
type tokenMatcher struct {
	accessToken string
}

func (m tokenMatcher) Matches(i any) bool {
	tokenStr, ok := i.(string)
	if !ok {
		return false
	}
	token := oauth2.Token{}
	err := json.Unmarshal([]byte(tokenStr), &token)
	if err != nil {
		return false
	}

	return token.AccessToken == m.accessToken
}

func (m tokenMatcher) String() string {
	return fmt.Sprintf("is equal to %s", m.accessToken)
}

func EqToken(accessToken string) gomock.Matcher {
	return tokenMatcher{accessToken}
}

func TestGetOIDCRedirectionURL(t *testing.T) {
	testCases := []struct {
		name                string
		config              map[string]any
		pkceVerifier        string
		expectedURL         string
		verifyPKCEChallenge bool
	}{
		{
			name: "Basic URL without PKCE",
			config: map[string]any{
				"authEndpoint": "https://provider.example.com/auth",
				"clientId":     "test-client-id",
				"rancherUrl":   "https://rancher.example.com/callback",
			},
			pkceVerifier: "",
			expectedURL:  "https://provider.example.com/auth?client_id=test-client-id&response_type=code&redirect_uri=https://rancher.example.com/callback",
		},
		{
			name: "With PKCE S256 method",
			config: map[string]any{
				"authEndpoint": "https://provider.example.com/auth",
				"clientId":     "test-client-id",
				"rancherUrl":   "https://rancher.example.com/callback",
				"pkceMethod":   "S256",
			},
			pkceVerifier:        "test-verifier-12345",
			verifyPKCEChallenge: true,
		},
		{
			name: "With PKCE plain method but no verifier provided",
			config: map[string]any{
				"authEndpoint": "https://provider.example.com/auth",
				"clientId":     "test-client-id",
				"rancherUrl":   "https://rancher.example.com/callback",
				"pkceMethod":   "plain",
			},
			pkceVerifier: "",
			expectedURL:  "https://provider.example.com/auth?client_id=test-client-id&response_type=code&redirect_uri=https://rancher.example.com/callback",
		},
		{
			name: "With PKCE S256 method configured but no verifier provided",
			config: map[string]any{
				"authEndpoint": "https://provider.example.com/auth",
				"clientId":     "test-client-id",
				"rancherUrl":   "https://rancher.example.com/callback",
				"pkceMethod":   "S256",
			},
			pkceVerifier: "",
			expectedURL:  "https://provider.example.com/auth?client_id=test-client-id&response_type=code&redirect_uri=https://rancher.example.com/callback",
		},
		{
			name: "With invalid PKCE method does not add PKCE params",
			config: map[string]any{
				"authEndpoint": "https://provider.example.com/auth",
				"clientId":     "test-client-id",
				"rancherUrl":   "https://rancher.example.com/callback",
				"pkceMethod":   "invalid-method",
			},
			pkceVerifier: "test-verifier-12345",
			expectedURL:  "https://provider.example.com/auth?client_id=test-client-id&response_type=code&redirect_uri=https://rancher.example.com/callback",
		},
		{
			name: "With special characters in URLs",
			config: map[string]any{
				"authEndpoint": "https://provider.example.com/auth?extra=param",
				"clientId":     "test-client-id",
				"rancherUrl":   "https://rancher.example.com/callback?state=xyz",
			},
			pkceVerifier: "",
			expectedURL:  "https://provider.example.com/auth?extra=param?client_id=test-client-id&response_type=code&redirect_uri=https://rancher.example.com/callback?state=xyz",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			values := &orderedValues{}
			result := GetOIDCRedirectionURL(tc.config, tc.pkceVerifier, values)

			if tc.verifyPKCEChallenge {
				assert.Contains(t, result, "client_id=test-client-id")
				assert.Contains(t, result, "response_type=code")
				assert.Contains(t, result, "code_challenge=")
				assert.Contains(t, result, "code_challenge_method=S256")
				assert.Contains(t, result, "redirect_uri=https://rancher.example.com/callback")
				assert.True(t, strings.HasPrefix(result, "https://provider.example.com/auth?"))
			} else {
				assert.Equal(t, tc.expectedURL, result)
			}
		})
	}
}
