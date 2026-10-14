package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/audit"
	authcontext "github.com/rancher/rancher/pkg/auth/context"
	requestsar "github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestAuthenticatedFilterWithSubjectAccessReviewHandler(t *testing.T) {
	t.Parallel()

	resourceAttributes := &authv1.ResourceAttributes{
		Verb:     "get",
		Resource: "clusters",
		Name:     "c-1",
	}

	var nextCalled atomic.Bool
	client := filterTestSubjectAccessReviewClient{
		createFunc: func(ctx context.Context, review *authv1.SubjectAccessReview, opts metav1.CreateOptions) (*authv1.SubjectAccessReview, error) {
			assert.Equal(t, metav1.CreateOptions{}, opts)
			assert.Equal(t, "test-user", review.Spec.User)
			// This is the bug this should not be using the headers from Impersonate-Groups!
			assert.Equal(t, []string{"group-a", "group-b"}, review.Spec.Groups)
			if assert.NotNil(t, review.Spec.ResourceAttributes) {
				assert.Equal(t, *resourceAttributes, *review.Spec.ResourceAttributes)
			}

			return &authv1.SubjectAccessReview{
				Status: authv1.SubjectAccessReviewStatus{Allowed: true},
			}, nil
		},
	}

	handler := NewAuthenticatedFilter(
		requestsar.NewSubjectAccessReviewHandler(client, resourceAttributes)(
			http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				nextCalled.Store(true)
				rw.WriteHeader(http.StatusPaymentRequired)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/clusters/c-1", nil)
	req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{
		Name:   "test-user",
		Groups: []string{"group-a", "group-b"},
	}))
	// This overrides the groups provided by the user.
	req.Header["Impersonate-Groups"] = []string{"group-c", "group-d"}
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.True(t, nextCalled.Load())
}

func TestAuthHeaderHandlerServeHTTPUnauthorized(t *testing.T) {
	runUnauthorizedCase := func(t *testing.T, req *http.Request) {
		t.Helper()

		var nextCalled atomic.Bool
		h := authHeaderHandler{next: http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			nextCalled.Store(true)
			rw.WriteHeader(http.StatusNoContent)
		})}

		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		assert.Equal(t, http.StatusUnauthorized, rw.Code)
		assert.False(t, nextCalled.Load())
		assert.Contains(t, rw.Body.String(), ErrMustAuthenticate.Error())
	}

	t.Run("request has no authenticated user", func(t *testing.T) {
		runUnauthorizedCase(t, httptest.NewRequest(http.MethodGet, "/", nil))
	})

	t.Run("authenticated user is system cattle error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := request.WithUser(req.Context(), &user.DefaultInfo{Name: "system:cattle:error"})
		runUnauthorizedCase(t, req.WithContext(ctx))
	})
}

func TestAuthHeaderHandlerServeHTTPSetsImpersonationHeadersForAuthenticatedUser(t *testing.T) {
	t.Parallel()

	type requestedHeaders struct {
		impersonateUser      string
		impersonateGroups    []string
		impersonateExtraFoo  []string
		invalidExtraObserved []string
	}

	var requested requestedHeaders
	h := authHeaderHandler{next: http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		requested = requestedHeaders{
			impersonateUser:      req.Header.Get("Impersonate-User"),
			impersonateGroups:    req.Header.Values("Impersonate-Group"),
			impersonateExtraFoo:  req.Header.Values("Impersonate-Extra-foo"),
			invalidExtraObserved: req.Header.Values("Impersonate-Extra-InvalidKey"),
		}
	})}

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(
		request.WithUser(t.Context(), &user.DefaultInfo{
			Name:   "test-user",
			Groups: []string{"group-a", "group-b"},
			Extra: map[string][]string{
				"foo": {"bar", ""},
			},
		}))

	req.Header.Set("Impersonate-User", "should-be-overwritten")
	req.Header.Add("Impersonate-Group", "should-be-replaced")
	req.Header.Add("Impersonate-Extra-InvalidKey", "must-be-removed")

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "test-user", requested.impersonateUser)
	assert.Equal(t, []string{"group-a", "group-b"}, requested.impersonateGroups)
	assert.Equal(t, []string{"bar"}, requested.impersonateExtraFoo)
	assert.Empty(t, requested.invalidExtraObserved)
}

func TestAuthHeaderHandlerServeHTTPClearsImpersonationHeadersForServiceAccountAuth(t *testing.T) {
	t.Parallel()

	type requestedHeaders struct {
		impersonateUser   string
		impersonateGroups []string
		impersonateFoo    []string
	}

	var requested requestedHeaders
	h := authHeaderHandler{next: http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		requested = requestedHeaders{
			impersonateUser:   req.Header.Get("Impersonate-User"),
			impersonateGroups: req.Header.Values("Impersonate-Group"),
			impersonateFoo:    req.Header.Values("Impersonate-Extra-foo"),
		}
	})}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = func(req *http.Request) *http.Request {
		ctx := request.WithUser(req.Context(), &user.DefaultInfo{
			Name:   "service-account-user",
			Groups: []string{"sa-group"},
			Extra: map[string][]string{
				"foo": {"bar"},
			},
		})
		ctx = authcontext.SetSAAuthenticated(ctx)
		return req.WithContext(ctx)
	}(req)

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Empty(t, requested.impersonateUser)
	assert.Empty(t, requested.impersonateGroups)
	assert.Empty(t, requested.impersonateFoo)
}

func TestAuthHeaderHandlerServeHTTPRefreshesAuditUsername(t *testing.T) {
	t.Parallel()

	realCaller := &user.DefaultInfo{
		Name:   "user-admin",
		Groups: []string{"system:authenticated"},
		Extra:  map[string][]string{"username": {"admin"}},
	}

	tests := []struct {
		name         string
		effective    *user.DefaultInfo
		wantName     string
		wantUsername string
	}{
		{
			name:         "request user unchanged",
			effective:    realCaller,
			wantName:     "user-admin",
			wantUsername: "admin",
		},
		{
			name: "impersonated user without username does not keep the real caller's username",
			effective: &user.DefaultInfo{
				Name:   "user-target",
				Groups: []string{"system:authenticated"},
			},
			wantName:     "user-target",
			wantUsername: "",
		},
		{
			name: "impersonated user with username",
			effective: &user.DefaultInfo{
				Name:   "user-target",
				Groups: []string{"system:authenticated"},
				Extra:  map[string][]string{"username": {"target"}},
			},
			wantName:     "user-target",
			wantUsername: "target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			writer, err := audit.NewWriter(out, audit.WriterOptions{})
			require.NoError(t, err)

			// Stands in for the impersonation filter, which replaces the request user after the audit user is captured.
			swapUser := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					next.ServeHTTP(rw, req.WithContext(request.WithUser(req.Context(), tt.effective)))
				})
			}
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			handler := audit.NewAuditLogMiddleware(writer)(swapUser(authHeaderHandler{next: next}))

			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(request.WithUser(t.Context(), realCaller))
			handler.ServeHTTP(httptest.NewRecorder(), req)

			var entry struct {
				User map[string]any `json:"user"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &entry))

			assert.Equal(t, tt.wantName, entry.User["name"])
			if tt.wantUsername == "" {
				assert.NotContains(t, entry.User, "username")
			} else {
				assert.Equal(t, tt.wantUsername, entry.User["username"])
			}
		})
	}
}

type filterTestAuthenticator struct {
	resp *AuthenticatorResponse
}

func (a filterTestAuthenticator) Authenticate(*http.Request) (*AuthenticatorResponse, error) {
	return a.resp, nil
}

func (a filterTestAuthenticator) TokenFromRequest(*http.Request) (accessor.TokenAccessor, error) {
	return nil, nil
}

func TestAuditUsernameFromAuthenticatorResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resp         *AuthenticatorResponse
		swapTo       *user.DefaultInfo // Stands in for the impersonation filter.
		wantName     string
		wantUsername string
	}{
		{
			name: "explicit username wins over the extra",
			resp: &AuthenticatorResponse{
				IsAuthed: true, User: "u-1",
				Extras:   map[string][]string{"username": {"from-extra"}},
				Username: "explicit",
			},
			wantName:     "u-1",
			wantUsername: "explicit",
		},
		{
			name: "no explicit username uses the extra",
			resp: &AuthenticatorResponse{
				IsAuthed: true, User: "u-1",
				Extras: map[string][]string{"username": {"from-extra"}},
			},
			wantName:     "u-1",
			wantUsername: "from-extra",
		},
		{
			name: "explicit username is dropped when the request user is replaced",
			resp: &AuthenticatorResponse{
				IsAuthed: true, User: "u-1",
				Extras:   map[string][]string{"username": {"from-extra"}},
				Username: "explicit",
			},
			swapTo:       &user.DefaultInfo{Name: "u-target"},
			wantName:     "u-target",
			wantUsername: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			writer, err := audit.NewWriter(out, audit.WriterOptions{})
			require.NoError(t, err)

			swap := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					if tt.swapTo != nil {
						req = req.WithContext(request.WithUser(req.Context(), tt.swapTo))
					}
					next.ServeHTTP(rw, req)
				})
			}
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			handler := ToAuthMiddleware(filterTestAuthenticator{resp: tt.resp})(
				audit.NewAuditLogMiddleware(writer)(swap(authHeaderHandler{next: next})))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

			var entry struct {
				User map[string]any `json:"user"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &entry))

			assert.Equal(t, tt.wantName, entry.User["name"])
			if tt.wantUsername == "" {
				assert.NotContains(t, entry.User, "username")
			} else {
				assert.Equal(t, tt.wantUsername, entry.User["username"])
			}
		})
	}
}

type filterTestSubjectAccessReviewClient struct {
	createFunc func(ctx context.Context, review *authv1.SubjectAccessReview, opts metav1.CreateOptions) (*authv1.SubjectAccessReview, error)
}

func (s filterTestSubjectAccessReviewClient) Create(ctx context.Context, review *authv1.SubjectAccessReview, opts metav1.CreateOptions) (*authv1.SubjectAccessReview, error) {
	return s.createFunc(ctx, review, opts)
}
