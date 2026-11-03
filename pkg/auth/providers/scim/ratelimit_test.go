package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	configFunc := func(r, b int) func(string) (int, int) {
		return func(string) (int, int) { return r, b }
	}

	t.Run("allows requests when disabled", func(t *testing.T) {
		t.Parallel()
		rl := newRateLimiter(configFunc(0, 10))
		handler := rl.Wrap(next)

		for range 100 {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("basic rate limiting", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			rl := newRateLimiter(configFunc(1, 1))
			handler := rl.Wrap(next)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusOK, w.Code)

			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
		})
	})

	t.Run("burst allowance", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			rl := newRateLimiter(configFunc(1, 5))
			handler := rl.Wrap(next)

			for i := range 5 {
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
				r.SetPathValue("provider", "okta")
				handler(w, r)
				assert.Equalf(t, http.StatusOK, w.Code, "request %d should be allowed", i+1)
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusTooManyRequests, w.Code, "6th request should be rejected")
		})
	})

	t.Run("per-provider isolation", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			rl := newRateLimiter(configFunc(1, 1))
			handler := rl.Wrap(next)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusOK, w.Code)

			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/v1-scim/azuread/Users", nil)
			r.SetPathValue("provider", "azuread")
			handler(w, r)
			assert.Equal(t, http.StatusOK, w.Code, "different provider should have independent limiter")

			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusTooManyRequests, w.Code, "okta should be rate limited")
		})
	})

	t.Run("per-provider config", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			rl := newRateLimiter(func(provider string) (int, int) {
				if provider == "okta" {
					return 1, 1
				}
				return 0, 10 // disabled for others
			})
			handler := rl.Wrap(next)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusOK, w.Code)

			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusTooManyRequests, w.Code, "okta should be rate limited")

			for range 10 {
				w = httptest.NewRecorder()
				r = httptest.NewRequest(http.MethodGet, "/v1-scim/azuread/Users", nil)
				r.SetPathValue("provider", "azuread")
				handler(w, r)
				assert.Equal(t, http.StatusOK, w.Code, "azuread should not be rate limited")
			}
		})
	})

	t.Run("config change propagation", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			currentRate := 1
			rl := newRateLimiter(func(string) (int, int) { return currentRate, 1 })
			handler := rl.Wrap(next)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusOK, w.Code)

			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusTooManyRequests, w.Code)

			currentRate = 0
			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusOK, w.Code, "disabling rate limit should allow requests")
		})
	})

	t.Run("429 response format", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			rl := newRateLimiter(configFunc(1, 1))
			handler := rl.Wrap(next)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)

			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)

			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			assert.Equal(t, "application/scim+json", w.Header().Get("Content-Type"))
			assert.Equal(t, "1", w.Header().Get("Retry-After"))

			var body Error
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, http.StatusTooManyRequests, body.Status)
			assert.Contains(t, body.Detail, "Rate limit exceeded")
			assert.Contains(t, body.Schemas, errorSchemaID)
		})
	})

	t.Run("negative burst clamped to 1", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			rl := newRateLimiter(configFunc(1, -5))
			handler := rl.Wrap(next)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusOK, w.Code)

			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/v1-scim/okta/Users", nil)
			r.SetPathValue("provider", "okta")
			handler(w, r)
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
		})
	})

	t.Run("concurrent access", func(t *testing.T) {
		t.Parallel()

		rl := newRateLimiter(configFunc(100, 100))
		handler := rl.Wrap(next)

		var wg sync.WaitGroup
		providers := []string{"okta", "azuread", "ping", "keycloak"}

		for _, p := range providers {
			for range 50 {
				wg.Go(func() {
					w := httptest.NewRecorder()
					r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+p+"/Users", nil)
					r.SetPathValue("provider", p)
					handler(w, r)
					assert.Equal(t, http.StatusOK, w.Code)
				})
			}
		}

		wg.Wait()
	})
}
