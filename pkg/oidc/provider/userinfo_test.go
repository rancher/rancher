package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/oidc/mocks"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		fakeGroupName  = "group-name"
	)
	fakeUser := v3.User{
		DisplayName: fakeUserName,
	}
	fakeUserAttributes := v3.UserAttribute{
		GroupPrincipals: map[string]v3.Principals{
			"group": {
				Items: []v3.Principal{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeGroupName,
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
		"auth_provider": "auth-provider",
		"scope":         []string{"openid", "profile"},
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
		"auth_provider": "auth-provider",
		"scope":         []string{"openid"},
	})
	fakeAccessTokenNoProfile.Header["kid"] = fakeSigningKey
	fakeAccessTokenNoProfileString, _ := fakeAccessTokenNoProfile.SignedString(privateKey)

	tests := map[string]struct {
		req          func() *http.Request
		mockSetup    func(mockParams)
		wantResponse *UserInfoResponse
		wantError    string
	}{
		"success response": {
			req: func() *http.Request {
				req, _ := http.NewRequest("GET", "https://rancher.com", nil)
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
				req, _ := http.NewRequest("GET", "https://rancher.com", nil)
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
		"invalid signature": {
			req: func() *http.Request {
				anotherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				accessTokenString, _ := fakeAccessToken.SignedString(anotherKey)
				req, _ := http.NewRequest("GET", "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessTokenString))
				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
			},
			wantError: `{"error":"invalid_request","error_description":"invalid access_token: token signature is invalid: crypto/rsa: verification error"}`,
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
				req, _ := http.NewRequest("GET", "https://rancher.com", nil)
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessTokenString))
				return req
			},
			mockSetup: func(mockParams mockParams) {
				mockParams.signingKeyGetter.EXPECT().GetPublicKey(fakeSigningKey).Return(&privateKey.PublicKey, nil)
			},
			wantError: `{"error":"invalid_request","error_description":"invalid access_token: it doesn't have scope"}`,
		},
		"no access token": {
			req: func() *http.Request {
				req, _ := http.NewRequest("GET", "https://rancher.com", nil)
				return req
			},
			wantError: `{"error":"invalid_request","error_description":"invalid access_token: token is malformed: token contains an invalid number of segments"}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
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
			}
			rec := httptest.NewRecorder()

			h.userInfoEndpoint(rec, test.req())

			if test.wantError != "" {
				assert.JSONEq(t, test.wantError, strings.TrimSpace(rec.Body.String()))
			} else {
				var response UserInfoResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				assert.NoError(t, err)
			}
		})
	}
}
