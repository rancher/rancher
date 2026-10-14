package requests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/requests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// TestImpersonationAuditUser runs requests through the audit middleware, the impersonation filter and
// NewAuthenticatedFilter, as the multi-cluster-manager routes do, and checks the written audit user.
func TestImpersonationAuditUser(t *testing.T) {
	t.Parallel()

	const (
		caller = "u-caller"
		target = "u-target"
		sa     = "system:serviceaccount:example-ns:example-sa"
	)
	callerGroups := []string{"system:authenticated", "caller-group"}
	newCaller := func() *user.DefaultInfo {
		return &user.DefaultInfo{
			Name:   caller,
			UID:    caller,
			Groups: append([]string(nil), callerGroups...),
			Extra:  map[string][]string{"username": {"alice"}},
		}
	}
	// The user records the audit log looks names up in when the entry has none.
	resolver := audit.WithUserResolver(func(id string) (string, string, bool) {
		if id == target {
			return "bob", "Bob Target", true
		}
		return "", "", false
	})

	tests := []struct {
		name            string
		impersonateUser string
		sar             func(*mocks.MockSubjectAccessReview)
		options         audit.WriterOptions
		wantStatus      int
		wantUser        map[string]any
	}{
		{
			name:       "no impersonation",
			wantStatus: http.StatusOK,
			wantUser: map[string]any{
				"name":     caller,
				"username": "alice",
				"group":    []any{"system:authenticated", "caller-group"},
				"extra":    map[string]any{"username": []any{"alice"}},
			},
		},
		{
			name:            "user impersonation records the target and the real caller",
			impersonateUser: target,
			sar: func(m *mocks.MockSubjectAccessReview) {
				m.EXPECT().UserCanImpersonateUser(gomock.Any(), caller, callerGroups, target).Return(true, nil)
			},
			wantStatus: http.StatusOK,
			wantUser: map[string]any{
				"name":     target,
				"username": "bob", // Resolved from the user record, not carried over from the caller.
				"group":    []any{"system:authenticated"},
				// Extra is captured at entry and is not refreshed, exactly as before.
				"extra": map[string]any{"username": []any{"alice"}},
				"impersonatedBy": map[string]any{
					"name":     caller,
					"username": "alice",
					"group":    []any{"system:authenticated", "caller-group"},
				},
				"requestUser": target,
			},
		},
		{
			name:            "denied impersonation keeps the caller",
			impersonateUser: target,
			sar: func(m *mocks.MockSubjectAccessReview) {
				m.EXPECT().UserCanImpersonateUser(gomock.Any(), caller, callerGroups, target).Return(false, nil)
			},
			wantStatus: http.StatusForbidden,
			wantUser: map[string]any{
				"name":        caller,
				"username":    "alice",
				"group":       []any{"system:authenticated", "caller-group"},
				"extra":       map[string]any{"username": []any{"alice"}},
				"requestUser": target,
			},
		},
		{
			// The request user is not replaced for service account impersonation, so the audit user is
			// unchanged from today: the caller stays the identity and there is no impersonatedBy.
			name:            "service account impersonation is unchanged",
			impersonateUser: sa,
			sar: func(m *mocks.MockSubjectAccessReview) {
				m.EXPECT().UserCanImpersonateServiceAccount(gomock.Any(), caller, callerGroups, sa).Return(true, nil)
			},
			wantStatus: http.StatusOK,
			wantUser: map[string]any{
				"name":        caller,
				"username":    "alice",
				"group":       []any{"system:authenticated", "caller-group"},
				"extra":       map[string]any{"username": []any{"alice"}},
				"requestUser": sa,
			},
		},
		{
			name:            "exclude groups also clears the impersonator's groups",
			impersonateUser: target,
			sar: func(m *mocks.MockSubjectAccessReview) {
				m.EXPECT().UserCanImpersonateUser(gomock.Any(), caller, callerGroups, target).Return(true, nil)
			},
			options:    audit.WriterOptions{ExcludeGroups: true},
			wantStatus: http.StatusOK,
			wantUser: map[string]any{
				"name":     target,
				"username": "bob",
				"extra":    map[string]any{"username": []any{"alice"}},
				"impersonatedBy": map[string]any{
					"name":     caller,
					"username": "alice",
				},
				"requestUser": target,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sarMock := mocks.NewMockSubjectAccessReview(gomock.NewController(t))
			if tt.sar != nil {
				tt.sar(sarMock)
			}

			out := &bytes.Buffer{}
			writer, err := audit.NewWriter(out, tt.options)
			require.NoError(t, err)

			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			ia := &ImpersonatingAuth{sar: sarMock}
			handler := audit.NewAuditLogMiddleware(writer, resolver)(
				ia.ImpersonationMiddleware(NewAuthenticatedFilter(next)))

			req := httptest.NewRequest(http.MethodGet, "/k8s/clusters/c-1/api", nil)
			req = req.WithContext(request.WithUser(req.Context(), newCaller()))
			if tt.impersonateUser != "" {
				req.Header.Set("Impersonate-User", tt.impersonateUser)
			}
			rw := httptest.NewRecorder()
			handler.ServeHTTP(rw, req)

			assert.Equal(t, tt.wantStatus, rw.Code)

			var entry struct {
				User map[string]any `json:"user"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &entry))
			assert.Equal(t, tt.wantUser, entry.User)
		})
	}
}
