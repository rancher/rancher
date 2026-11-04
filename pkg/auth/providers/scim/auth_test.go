package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/rancher/rancher/pkg/auth/scimconfig"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func enabledProvider(string) scimconfig.Config { return scimconfig.Config{Enabled: true} }

func TestTokenAuthenticator(t *testing.T) {
	ctrl := gomock.NewController(t)

	validToken1 := "ebbebf0873ed0935e0e0e506fc5065dc391450d14cdddd7de4f677868a88c486"
	validToken2 := "f3104cc584da56ef4a1bd0e019845f4423c0d853a2d5329abfea34977cb8cfed"

	provider := "okta"
	isDisabledProvider := func(p string) (bool, error) {
		return p != local.Name && p != provider, nil
	}
	wantSelector := labels.Set{
		secretKindLabel:   scimAuthToken,
		authProviderLabel: provider,
	}.AsSelector()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// matchingSecretCache returns a Secret cache that lists one token Secret holding validToken1
	// for the given provider.
	matchingSecretCache := func(t *testing.T, provider string) *fake.MockCacheInterface[*v1.Secret] {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, labels.Set{secretKindLabel: scimAuthToken, authProviderLabel: provider}.AsSelector().String(), selector.String())

			return []*v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "scim-" + provider},
					Data:       map[string][]byte{"token": []byte(validToken1)},
				},
			}, nil
		}).Times(1)

		return secretCache
	}

	t.Run("single secret", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), selector.String())

			return []*v1.Secret{
				{
					Data: map[string][]byte{
						"token": []byte(validToken1),
					},
				},
			}, nil
		}).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return 0 },
			getConfig:          enabledProvider,
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	})

	t.Run("multiple secrets", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), selector.String())

			return []*v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "valid-token-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
					},
					Data: map[string][]byte{
						"token": []byte(validToken1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "valid-token-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
					},
					Data: map[string][]byte{
						"token": []byte(validToken2),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "expired-token",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
					},
					Data: map[string][]byte{
						"token": []byte(validToken2),
					},
				},
			}, nil
		}).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return time.Hour },
			getConfig:          enabledProvider,
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken2)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	})

	t.Run("no auth header", func(t *testing.T) {
		auth := &tokenAuthenticator{}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("not a bearer token", func(t *testing.T) {
		auth := &tokenAuthenticator{}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("unknown provider", func(t *testing.T) {
		auth := &tokenAuthenticator{
			secretCache: matchingSecretCache(t, provider),
			isDisabledProvider: func(p string) (bool, error) {
				return false, fmt.Errorf("unknown provider")
			},
			expireTokensAfter: func() time.Duration { return 0 },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})

	t.Run("local provider", func(t *testing.T) {
		provider := local.Name
		auth := &tokenAuthenticator{
			secretCache:       matchingSecretCache(t, provider),
			expireTokensAfter: func() time.Duration { return 0 },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})

	t.Run("disabled provider", func(t *testing.T) {
		auth := &tokenAuthenticator{
			secretCache: matchingSecretCache(t, provider),
			isDisabledProvider: func(p string) (bool, error) {
				return true, nil
			},
			expireTokensAfter: func() time.Duration { return 0 },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})

	t.Run("no secrets", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*v1.Secret{}, nil).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return 0 },
			getConfig:          enabledProvider,
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("no secrets with matching token", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), selector.String())

			return []*v1.Secret{
				{
					Data: map[string][]byte{
						"token": []byte(validToken1),
					},
				},
				{
					Data: map[string][]byte{
						"token": []byte(validToken2),
					},
				},
			}, nil
		}).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return 0 },
			getConfig:          enabledProvider,
		}

		someOtherToken := "c4faf0453d39dffa3bf7d3135f6a15e50dbd4b71fe74c5d5b9d45772e36511e1"

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+someOtherToken)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("single expired token", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)

		expiredSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "expired-token",
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
			},
			Data: map[string][]byte{
				"token": []byte(validToken1),
			},
		}

		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), selector.String())
			return []*v1.Secret{expiredSecret}, nil
		}).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return time.Hour },
			getConfig:          enabledProvider,
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("all tokens expired", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)

		expiredSecret1 := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "expired-token-1",
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
			},
			Data: map[string][]byte{
				"token": []byte(validToken1),
			},
		}

		expiredSecret2 := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "expired-token-2",
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-3 * time.Hour)},
			},
			Data: map[string][]byte{
				"token": []byte(validToken2),
			},
		}

		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), selector.String())
			return []*v1.Secret{expiredSecret1, expiredSecret2}, nil
		}).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return time.Hour },
			getConfig:          enabledProvider,
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("scim not enabled", func(t *testing.T) {
		auth := &tokenAuthenticator{
			secretCache:        matchingSecretCache(t, provider),
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return 0 },
			getConfig:          func(string) scimconfig.Config { return scimconfig.Config{Enabled: false} },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})

	t.Run("scim paused", func(t *testing.T) {
		auth := &tokenAuthenticator{
			secretCache:        matchingSecretCache(t, provider),
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return 0 },
			getConfig:          func(string) scimconfig.Config { return scimconfig.Config{Enabled: true, Paused: true} },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusServiceUnavailable, w.Result().StatusCode)
	})
}

func TestTokenAuthenticatorListError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
	secretCache.EXPECT().List(tokenSecretNamespace, gomock.Any()).Return(nil, fmt.Errorf("cache not ready")).Times(1)

	auth := &tokenAuthenticator{
		secretCache: secretCache,
		isDisabledProvider: func(string) (bool, error) {
			t.Error("isDisabledProvider called after a list error")
			return false, nil
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/scim/okta/Users", nil)
	r.SetPathValue("provider", "okta")
	r.Header.Set("Authorization", "Bearer token")

	auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler called after a list error")
	})).ServeHTTP(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
}

func TestTokenAuthenticatorInvalidToken(t *testing.T) {
	t.Parallel()

	const (
		validToken   = "ebbebf0873ed0935e0e0e506fc5065dc391450d14cdddd7de4f677868a88c486"
		invalidToken = "c4faf0453d39dffa3bf7d3135f6a15e50dbd4b71fe74c5d5b9d45772e36511e1"
	)

	tests := []struct {
		name               string
		provider           string
		isDisabledProvider func(string) (bool, error)
		config             scimconfig.Config
	}{
		{
			name:               "provider doesn't exist",
			provider:           "okta",
			isDisabledProvider: func(string) (bool, error) { return false, fmt.Errorf("not found") },
			config:             scimconfig.Config{Enabled: true},
		},
		{
			name:               "provider disabled",
			provider:           "okta",
			isDisabledProvider: func(string) (bool, error) { return true, nil },
			config:             scimconfig.Config{Enabled: true},
		},
		{
			name:               "local provider",
			provider:           local.Name,
			isDisabledProvider: func(string) (bool, error) { return false, nil },
			config:             scimconfig.Config{Enabled: true},
		},
		{
			name:               "scim not enabled",
			provider:           "okta",
			isDisabledProvider: func(string) (bool, error) { return false, nil },
			config:             scimconfig.Config{Enabled: false},
		},
		{
			name:               "scim paused",
			provider:           "okta",
			isDisabledProvider: func(string) (bool, error) { return false, nil },
			config:             scimconfig.Config{Enabled: true, Paused: true},
		},
		{
			name:               "scim enabled",
			provider:           "okta",
			isDisabledProvider: func(string) (bool, error) { return false, nil },
			config:             scimconfig.Config{Enabled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
			secretCache.EXPECT().List(tokenSecretNamespace, gomock.Any()).Return([]*v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "scim-" + tt.provider},
					Data:       map[string][]byte{"token": []byte(validToken)},
				},
			}, nil).Times(1)

			// The stubs return the provider's state, so that checking it before the token gives a
			// different response, and fail the test because nothing but the token should be read.
			auth := &tokenAuthenticator{
				secretCache: secretCache,
				isDisabledProvider: func(provider string) (bool, error) {
					t.Error("isDisabledProvider called for an invalid token")
					return tt.isDisabledProvider(provider)
				},
				expireTokensAfter: func() time.Duration {
					t.Error("expireTokensAfter called for an invalid token")
					return 0
				},
				getConfig: func(string) scimconfig.Config {
					t.Error("getConfig called for an invalid token")
					return tt.config
				},
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+tt.provider+"/Users", nil)
			r.SetPathValue("provider", tt.provider)
			r.Header.Set("Authorization", "Bearer "+invalidToken)

			auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("next handler called for an invalid token")
			})).ServeHTTP(w, r)

			assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
		})
	}
}

func TestTokenAuthenticatorAuditUser(t *testing.T) {
	t.Parallel()

	const (
		provider   = "okta"
		validToken = "ebbebf0873ed0935e0e0e506fc5065dc391450d14cdddd7de4f677868a88c486"
		otherToken = "f3104cc584da56ef4a1bd0e019845f4423c0d853a2d5329abfea34977cb8cfed"
		patchBody  = `{"Operations":[{"op":"replace","path":"active","value":false}]}`
	)

	secrets := []*v1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "scim-okta-new"}, Data: map[string][]byte{"token": []byte(validToken)}},
		{ObjectMeta: metav1.ObjectMeta{Name: "scim-okta-old"}, Data: map[string][]byte{"token": []byte(otherToken)}},
	}

	// serve runs a request through the audit log middleware and the authenticator,
	// as in production, and returns the written audit log entry. Usernames are on, because that is the
	// case in which the request user's identity could leak into the SCIM entry.
	// serveWithTTL is serve with token Secrets expiring after ttl.
	serveWithTTL := func(t *testing.T, secrets []*v1.Secret, ttl time.Duration, token string, opts audit.WriterOptions, userInfo *user.DefaultInfo) map[string]any {
		ctrl := gomock.NewController(t)
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil).AnyTimes()

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			isDisabledProvider: func(string) (bool, error) { return false, nil },
			expireTokensAfter:  func() time.Duration { return ttl },
			getConfig:          enabledProvider,
		}

		out := &bytes.Buffer{}
		writer, err := audit.NewWriter(out, opts)
		require.NoError(t, err)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := io.ReadAll(r.Body)
			require.NoError(t, err)
		})
		handler := audit.NewAuditLogMiddleware(writer, audit.WithIncludeUsername(func() bool { return true }))(auth.Authenticate(next))

		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/u-abc", strings.NewReader(patchBody))
		r.Header.Set("Content-Type", "application/scim+json")
		r.SetPathValue("provider", provider)
		r.Header.Set("Authorization", "Bearer "+token)
		r = r.WithContext(request.WithUser(r.Context(), userInfo))

		handler.ServeHTTP(httptest.NewRecorder(), r)

		var entry map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &entry))
		return entry
	}

	serve := func(t *testing.T, secrets []*v1.Secret, token string, opts audit.WriterOptions, userInfo *user.DefaultInfo) map[string]any {
		return serveWithTTL(t, secrets, 0, token, opts, userInfo)
	}

	// newErrorUser returns the user Rancher's authenticator sets for a SCIM token it doesn't recognize.
	// Groups has spare capacity so an append would write into it.
	newErrorUser := func() *user.DefaultInfo {
		groups := make([]string, 2, 4)
		copy(groups, []string{"system:unauthenticated", "system:cattle:error"})
		return &user.DefaultInfo{
			Name:   "system:cattle:error",
			Groups: groups,
			Extra:  map[string][]string{"foo": {"bar"}},
		}
	}

	t.Run("valid token", func(t *testing.T) {
		t.Parallel()

		userInfo := newErrorUser()

		got := serve(t, secrets, validToken, audit.WriterOptions{}, userInfo)["user"]

		assert.Equal(t, map[string]any{
			"name":  "system:scim:okta",
			"group": []any{"system:scim"},
			"extra": map[string]any{
				"scim.cattle.io/provider": []any{"okta"},
				"scim.cattle.io/token-id": []any{"scim-okta-new"},
			},
		}, got)

		// The user info in the request context is unchanged.
		assert.Equal(t, newErrorUser(), userInfo)
		assert.Equal(t, []string{"system:unauthenticated", "system:cattle:error", "", ""}, userInfo.Groups[:4])
	})

	t.Run("valid token does not keep an identity from the request user", func(t *testing.T) {
		t.Parallel()

		userInfo := newErrorUser()
		userInfo.Extra = map[string][]string{"username": {"someone"}}

		got := serve(t, secrets, validToken, audit.WriterOptions{}, userInfo)["user"].(map[string]any)

		assert.Equal(t, "system:scim:okta", got["name"])
		assert.NotContains(t, got, "identity")
	})

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()

		got := serve(t, secrets, "invalid", audit.WriterOptions{}, newErrorUser())["user"].(map[string]any)

		assert.Equal(t, "system:cattle:error", got["name"])
		assert.Equal(t, []any{"system:unauthenticated", "system:cattle:error"}, got["group"])
		assert.Equal(t, map[string]any{"foo": []any{"bar"}}, got["extra"])
	})

	t.Run("exclude groups", func(t *testing.T) {
		t.Parallel()

		got := serve(t, secrets, validToken, audit.WriterOptions{ExcludeGroups: true}, newErrorUser())["user"].(map[string]any)

		assert.Equal(t, "system:scim:okta", got["name"])
		assert.NotContains(t, got, "group")
	})

	t.Run("valid token captures the request body", func(t *testing.T) {
		t.Parallel()

		got := serve(t, secrets, validToken, audit.WriterOptions{DefaultPolicyLevel: auditlogv1.LevelRequest}, newErrorUser())

		var want map[string]any
		require.NoError(t, json.Unmarshal([]byte(patchBody), &want))
		assert.Equal(t, want, got["requestBody"])
	})

	t.Run("invalid token does not mark the request", func(t *testing.T) {
		t.Parallel()

		got := serve(t, secrets, "invalid", audit.WriterOptions{DefaultPolicyLevel: auditlogv1.LevelRequestResponse}, newErrorUser())

		assert.Nil(t, got["requestBody"])
		// The 401 response is application/scim+json, which is logged only for marked requests.
		assert.Nil(t, got["responseBody"])
	})

	t.Run("matching secret listed second", func(t *testing.T) {
		t.Parallel()

		reversed := []*v1.Secret{secrets[1], secrets[0]}
		got := serve(t, reversed, validToken, audit.WriterOptions{}, newErrorUser())["user"].(map[string]any)

		assert.Equal(t, map[string]any{
			"scim.cattle.io/provider": []any{"okta"},
			"scim.cattle.io/token-id": []any{"scim-okta-new"},
		}, got["extra"])
	})

	t.Run("first secret with the same token is used", func(t *testing.T) {
		t.Parallel()

		sameToken := []*v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "scim-okta-first", CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute))},
				Data:       map[string][]byte{"token": []byte(validToken)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "scim-okta-second", CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute))},
				Data:       map[string][]byte{"token": []byte(validToken)},
			},
		}
		got := serveWithTTL(t, sameToken, time.Hour, validToken, audit.WriterOptions{}, newErrorUser())["user"].(map[string]any)

		assert.Equal(t, map[string]any{
			"scim.cattle.io/provider": []any{"okta"},
			"scim.cattle.io/token-id": []any{"scim-okta-first"},
		}, got["extra"])
	})

	t.Run("expired secret with the same token is skipped", func(t *testing.T) {
		t.Parallel()

		sameToken := []*v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "scim-okta-expired", CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour))},
				Data:       map[string][]byte{"token": []byte(validToken)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "scim-okta-current", CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute))},
				Data:       map[string][]byte{"token": []byte(validToken)},
			},
		}
		got := serveWithTTL(t, sameToken, time.Hour, validToken, audit.WriterOptions{}, newErrorUser())["user"].(map[string]any)

		assert.Equal(t, map[string]any{
			"scim.cattle.io/provider": []any{"okta"},
			"scim.cattle.io/token-id": []any{"scim-okta-current"},
		}, got["extra"])
	})
}

func TestIsDisabledProviderFromCache(t *testing.T) {
	t.Parallel()

	notFound := apierrors.NewNotFound(v3.Resource("authconfigs"), "okta")

	tests := []struct {
		name         string
		authConfig   *v3.AuthConfig
		getErr       error
		registered   bool
		wantDisabled bool
		wantErr      error
	}{
		{
			name:    "missing",
			getErr:  notFound,
			wantErr: notFound,
		},
		{
			name:         "disabled",
			authConfig:   &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "okta"}, Type: "oktaConfig", Enabled: false},
			registered:   true,
			wantDisabled: true,
		},
		{
			name:         "enabled with a registered type",
			authConfig:   &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "okta"}, Type: "oktaConfig", Enabled: true},
			registered:   true,
			wantDisabled: false,
		},
		{
			name:         "enabled with an unregistered type",
			authConfig:   &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "okta"}, Type: "oktaConfig", Enabled: true},
			registered:   false,
			wantDisabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			authConfigCache := fake.NewMockNonNamespacedCacheInterface[*v3.AuthConfig](ctrl)
			authConfigCache.EXPECT().Get("okta").Return(tt.authConfig, tt.getErr).Times(1)

			a := &tokenAuthenticator{
				authConfigCache: authConfigCache,
				getProviderByType: func(authConfigType string) common.AuthProvider {
					assert.Equal(t, "oktaConfig", authConfigType)
					if !tt.registered {
						return nil
					}
					return mocks.NewMockAuthProvider(ctrl)
				},
			}

			disabled, err := a.isDisabledProviderFromCache("okta")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDisabled, disabled)
		})
	}
}
