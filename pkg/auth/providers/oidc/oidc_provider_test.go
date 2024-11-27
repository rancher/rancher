package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/oauth2"
)

func Test_validateACR(t *testing.T) {
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
	header := base64.URLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	validClaims := base64.URLEncoding.EncodeToString([]byte(`{"acr":"example_acr"}`))
	invalidBase64Claims := "invalid_base64_claims"
	noAcrClaims := base64.URLEncoding.EncodeToString([]byte(`{"sub":"1234567890"}`))

	tests := []struct {
		name        string
		token       string
		expectedACR string
		wantError   bool
	}{
		{
			name:        "valid token with ACR",
			token:       fmt.Sprintf("%s.%s.", header, validClaims),
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
			token:       fmt.Sprintf("%s.%s.", header, noAcrClaims),
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
		config                    func(string) *v32.OIDCConfig
		authCode                  string
		claimInfo                 *ClaimInfo
		tokenManagerMock          func(token *Token) tokenManager
		oidcProviderResponses     func(string) oidcResponses
		expectedUserInfoSubject   string
		expectedUserInfoClaimInfo ClaimInfo
		expectedErrorMessage      string
	}{
		"token is updated and userInfo returned": {
			config: func(port string) *v32.OIDCConfig {
				return newOIDCContext(port)
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
				Subject:           "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
				PreferredUsername: "admin",
				EmailVerified:     true,
				Groups:            []string{"admingroup"},
				FullGroupPath:     []string{"/admingroup"},
			},
		},
		"error - invalid certificate": {
			config: func(port string) *v32.OIDCConfig {
				return &v32.OIDCConfig{
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
			config: func(port string) *v32.OIDCConfig {
				return newOIDCContext(port)
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
			config: func(port string) *v32.OIDCConfig {
				return newOIDCContext(port)
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
				TokenMGR: test.tokenManagerMock(oidcResp.token),
			}
			ctx := context.TODO()

			userInfo, token, err := o.getUserInfoFromAuthCode(&ctx, test.config(port), test.authCode, test.claimInfo, userId)

			if test.expectedErrorMessage != "" {
				assert.ErrorContains(t, err, test.expectedErrorMessage)
			} else {
				assert.NoError(t, err)
				claims := ClaimInfo{}
				assert.NoError(t, userInfo.Claims(&claims))
				assert.Equal(t, test.expectedUserInfoSubject, userInfo.Subject)
				assert.Equal(t, test.expectedUserInfoClaimInfo, claims)
				assert.Equal(t, oidcResp.token.AccessToken, token.AccessToken) //token should be the same as the one returned by the mock oidc server.
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
		config                func(string) *v32.OIDCConfig
		storedToken           func(string) *oauth2.Token
		tokenManagerMock      func(token *Token) tokenManager
		oidcProviderResponses func(string) oidcResponses
		expectedClaimInfo     *ClaimInfo
		expectedErrorMessage  string
	}{
		"get claims with valid token": {
			config: func(port string) *v32.OIDCConfig {
				return newOIDCContext(port)
			},
			storedToken: func(port string) *oauth2.Token {
				token := jwt.New(jwt.SigningMethodRS256)
				token.Claims = jwt.StandardClaims{
					Audience:  "test",
					ExpiresAt: time.Now().Add(5 * time.Minute).Unix(), // expires in the future
					Issuer:    "http://localhost:" + port,
				}
				tokenStr, err := token.SignedString(privateKey)
				assert.NoError(t, err)

				return &oauth2.Token{
					AccessToken: tokenStr,
					Expiry:      time.Now().Add(5 * time.Minute), // expires in the future
				}
			},
			oidcProviderResponses: func(port string) oidcResponses {
				return newOIDCResponses(privateKey, port)
			},
			tokenManagerMock: func(_ *Token) tokenManager {
				return mocks.NewMocktokenManager(ctrl)
			},
			expectedClaimInfo: &ClaimInfo{
				Subject:           "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
				PreferredUsername: "admin",
				EmailVerified:     true,
				Groups:            []string{"admingroup"},
				FullGroupPath:     []string{"/admingroup"},
			},
		},
		"token is refreshed and updated when expired": {
			config: func(port string) *v32.OIDCConfig {
				return newOIDCContext(port)
			},
			oidcProviderResponses: func(port string) oidcResponses {
				return newOIDCResponses(privateKey, port)
			},
			storedToken: func(port string) *oauth2.Token {
				token := jwt.New(jwt.SigningMethodRS256)
				token.Claims = jwt.StandardClaims{
					Audience:  "test",
					ExpiresAt: time.Unix(0, 0).Unix(), // has expired
					Issuer:    "http://localhost:" + port,
				}
				tokenStr, err := token.SignedString(privateKey)
				assert.NoError(t, err)
				refreshToken := jwt.New(jwt.SigningMethodRS256)
				refreshToken.Claims = jwt.StandardClaims{
					Audience:  "test",
					ExpiresAt: time.Now().Add(5 * time.Minute).Unix(), // expires in the future
					Issuer:    "http://localhost:" + port,
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
				Subject:           "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
				PreferredUsername: "admin",
				EmailVerified:     true,
				Groups:            []string{"admingroup"},
				FullGroupPath:     []string{"/admingroup"},
			},
			tokenManagerMock: func(token *Token) tokenManager {
				mock := mocks.NewMocktokenManager(ctrl)
				mock.EXPECT().UpdateSecret(userId, providerName, EqToken(token.RefreshToken))

				return mock
			},
		},
		"error - invalid certificate": {
			config: func(port string) *v32.OIDCConfig {
				return &v32.OIDCConfig{
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
			config: func(port string) *v32.OIDCConfig {
				return newOIDCContext(port)
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
			config: func(port string) *v32.OIDCConfig {
				return newOIDCContext(port)
			},
			storedToken: func(port string) *oauth2.Token {
				token := jwt.New(jwt.SigningMethodRS256)
				token.Claims = jwt.StandardClaims{
					Audience:  "test",
					ExpiresAt: time.Now().Add(5 * time.Minute).Unix(), // expires in the future
					Issuer:    "http://localhost:" + port,
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
				TokenMGR: test.tokenManagerMock(oidcResp.token),
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
	jwtToken.Claims = jwt.StandardClaims{
		Audience:  "test",
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(), // has expired
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
				"preferred_username": "admin"
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

func newOIDCContext(port string) *v32.OIDCConfig {
	return &v32.OIDCConfig{
		Issuer:           "http://localhost:" + port,
		ClientID:         "test",
		JWKSUrl:          "http://localhost:" + port + "/.well-known/jwks.json",
		AuthEndpoint:     "http://localhost:" + port + "/auth",
		TokenEndpoint:    "http://localhost:" + port + "/token",
		UserInfoEndpoint: "http://localhost:" + port + "/user",
	}
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

// expiryIn is calculated inside the oauth2 library using time.Now, so we just compare the token is equal
type tokenMatcher struct {
	accessToken string
}

func (m tokenMatcher) Matches(i interface{}) bool {
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
