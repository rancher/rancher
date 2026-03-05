package scim

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

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
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
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

		secrets := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().Delete(tokenSecretNamespace, "expired-token", nil).Return(nil).Times(1)

		auth := &tokenAuthenticator{
			secrets:            secrets,
			secretCache:        secretCache,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return time.Hour },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+validToken2)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	})

	t.Run("no auth header", func(t *testing.T) {
		auth := &tokenAuthenticator{}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("not a bearer token", func(t *testing.T) {
		auth := &tokenAuthenticator{}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("unknown provider", func(t *testing.T) {
		auth := &tokenAuthenticator{
			isDisabledProvider: func(p string) (bool, error) {
				return false, fmt.Errorf("unknown provider")
			},
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})

	t.Run("local provider", func(t *testing.T) {
		auth := &tokenAuthenticator{}

		provider := "local"
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})

	t.Run("disabled provider", func(t *testing.T) {
		auth := &tokenAuthenticator{
			isDisabledProvider: func(p string) (bool, error) {
				return true, nil
			},
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
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
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
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
		}

		someOtherToken := "c4faf0453d39dffa3bf7d3135f6a15e50dbd4b71fe74c5d5b9d45772e36511e1"

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+someOtherToken)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("single expired token", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secrets := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)

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

		secrets.EXPECT().Delete(tokenSecretNamespace, "expired-token", nil).Return(nil).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			secrets:            secrets,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return time.Hour },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("all tokens expired", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secrets := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)

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

		secrets.EXPECT().Delete(tokenSecretNamespace, "expired-token-1", nil).Return(nil).Times(1)
		secrets.EXPECT().Delete(tokenSecretNamespace, "expired-token-2", nil).Return(nil).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			secrets:            secrets,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return time.Hour },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("token deletion failure does not fail authentication", func(t *testing.T) {
		secretCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secrets := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)

		validSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "valid-token",
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
			},
			Data: map[string][]byte{
				"token": []byte(validToken1),
			},
		}

		expiredSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "expired-token",
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
			},
			Data: map[string][]byte{
				"token": []byte(validToken2),
			},
		}

		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, wantSelector.String(), selector.String())
			return []*v1.Secret{validSecret, expiredSecret}, nil
		}).Times(1)

		// Deletion fails but authentication should continue
		secrets.EXPECT().Delete(tokenSecretNamespace, "expired-token", nil).Return(fmt.Errorf("deletion failed")).Times(1)

		auth := &tokenAuthenticator{
			secretCache:        secretCache,
			secrets:            secrets,
			isDisabledProvider: isDisabledProvider,
			expireTokensAfter:  func() time.Duration { return time.Hour },
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		// Authentication should succeed despite deletion failure
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	})
}
