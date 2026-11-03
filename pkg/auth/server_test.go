package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// markerHandler writes its name so tests can tell which route was picked.
func markerHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(name))
	})
}

func testRoutes() authRoutes {
	return authRoutes{
		limit:    func(next http.Handler) http.Handler { return next },
		saml:     markerHandler("saml"),
		v1Public: markerHandler("v1Public"),
		v3:       markerHandler("v3"),
		logout:   markerHandler("logout"),
		next:     markerHandler("next"),
	}
}

func TestNewRootMux(t *testing.T) {
	t.Parallel()

	authenticated := &user.DefaultInfo{Name: "u-abcde", Groups: []string{"system:authenticated"}}

	tests := []struct {
		name     string
		method   string
		path     string
		user     user.Info
		wantCode int
		wantBody string
	}{
		{
			name:     "logout is served when MCM is disabled",
			method:   http.MethodPost,
			path:     "/v1/logout",
			user:     authenticated,
			wantCode: http.StatusOK,
			wantBody: "logout",
		},
		{
			name:     "logout all is served when MCM is disabled",
			method:   http.MethodPost,
			path:     "/v1/logout?all",
			user:     authenticated,
			wantCode: http.StatusOK,
			wantBody: "logout",
		},
		{
			name:     "logout requires authentication",
			method:   http.MethodPost,
			path:     "/v1/logout",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "logout rejects a failed authentication",
			method:   http.MethodPost,
			path:     "/v1/logout",
			user:     &user.DefaultInfo{Name: "system:cattle:error"},
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "a non POST logout falls through",
			method:   http.MethodGet,
			path:     "/v1/logout",
			user:     authenticated,
			wantCode: http.StatusOK,
			wantBody: "next",
		},
		{
			name:     "other v1 paths fall through",
			method:   http.MethodPost,
			path:     "/v1/management.cattle.io.settings",
			user:     authenticated,
			wantCode: http.StatusOK,
			wantBody: "next",
		},
		{
			name:     "v3 is served",
			method:   http.MethodGet,
			path:     "/v3/tokens",
			user:     authenticated,
			wantCode: http.StatusOK,
			wantBody: "v3",
		},
		{
			name:     "v1 public is served",
			method:   http.MethodGet,
			path:     "/v1-public/authProviders",
			wantCode: http.StatusOK,
			wantBody: "v1Public",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(test.method, test.path, nil)
			if test.user != nil {
				req = req.WithContext(request.WithUser(req.Context(), test.user))
			}

			rec := httptest.NewRecorder()
			newRootMux(testRoutes()).ServeHTTP(rec, req)

			require.Equal(t, test.wantCode, rec.Code)
			if test.wantBody != "" {
				assert.Equal(t, test.wantBody, rec.Body.String())
			}
		})
	}
}

func TestNewRootMuxOptionalRoutes(t *testing.T) {
	t.Parallel()

	t.Run("v3 public and scim fall through when not configured", func(t *testing.T) {
		t.Parallel()

		mux := newRootMux(testRoutes())

		for _, path := range []string{"/v3-public/authProviders", "/v1-scim/v2/Users"} {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			assert.Equal(t, "next", rec.Body.String(), "path %s", path)
		}
	})

	t.Run("v3 public and scim are served when configured", func(t *testing.T) {
		t.Parallel()

		routes := testRoutes()
		routes.v3Public = markerHandler("v3Public")
		routes.scim = markerHandler("scim")
		mux := newRootMux(routes)

		for path, want := range map[string]string{
			"/v3-public/authProviders": "v3Public",
			"/v1-scim/v2/Users":        "scim",
		} {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			assert.Equal(t, want, rec.Body.String(), "path %s", path)
		}
	})
}
