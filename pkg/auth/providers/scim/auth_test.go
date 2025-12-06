package scim

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestTokenAuthenticator(t *testing.T) {
	ctrl := gomock.NewController(t)

	validToken1 := "ebbebf0873ed0935e0e0e506fc5065dc391450d14cdddd7de4f677868a88c486"
	validToken2 := "f3104cc584da56ef4a1bd0e019845f4423c0d853a2d5329abfea34977cb8cfed"

	provider := "okta"
	isDisabledProvider := func(p string) (bool, error) {
		return p != provider, nil
	}
	wantSelector := labels.Set{
		secretKindLabel:   scimAuthToken,
		authProviderLabel: provider,
	}.AsSelector()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	t.Run("single secret", func(t *testing.T) {
		secrets := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
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
			secrets:            secrets,
			isDisabledProvider: isDisabledProvider,
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	})

	t.Run("multiple secrets", func(t *testing.T) {
		secrets := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
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
			secrets:            secrets,
			isDisabledProvider: isDisabledProvider,
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
		secrets := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*v1.Secret{}, nil).Times(1)

		auth := &tokenAuthenticator{
			secrets:            secrets,
			isDisabledProvider: isDisabledProvider,
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+validToken1)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("no secrets with matching token", func(t *testing.T) {
		secrets := fake.NewMockCacheInterface[*v1.Secret](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
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
			secrets:            secrets,
			isDisabledProvider: isDisabledProvider,
		}

		someOtherToken := "c4faf0453d39dffa3bf7d3135f6a15e50dbd4b71fe74c5d5b9d45772e36511e1"

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/scim/"+provider+"/Users", nil)
		r = mux.SetURLVars(r, map[string]string{"provider": provider})
		r.Header.Set("Authorization", "Bearer "+someOtherToken)

		auth.Authenticate(next).ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})
}
