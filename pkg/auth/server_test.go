package auth

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/impersonation"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		limit:         func(next http.Handler) http.Handler { return next },
		saml:          markerHandler("saml"),
		v1Public:      markerHandler("v1Public"),
		v3:            markerHandler("v3"),
		logout:        markerHandler("logout"),
		impersonation: &impersonation.Handler{},
		next:          markerHandler("next"),
	}
}

func TestNewRootMuxRecoveryRouteIsAvailableWithoutAuthentication(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
	rec := httptest.NewRecorder()

	newRootMux(testRoutes()).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"restored":false`)

	regular := httptest.NewRecorder()
	newRootMux(testRoutes()).ServeHTTP(regular, httptest.NewRequest(http.MethodPost, "/v1/logout", nil))
	require.Equal(t, http.StatusUnauthorized, regular.Code)
}

type routeRecoveryUsers map[string]*apiv3.User

func (u routeRecoveryUsers) Get(name string, _ metav1.GetOptions) (*apiv3.User, error) {
	user, ok := u[name]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

type routeRecoveryAttributes struct{}

func (routeRecoveryAttributes) Get(string, metav1.GetOptions) (*apiv3.UserAttribute, error) {
	return nil, apierrors.NewNotFound(apiv3.Resource("userattribute"), "u-admin")
}

type routeRecoveryBindings struct{}

func (routeRecoveryBindings) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	return &apiv3.GlobalRoleBindingList{Items: []apiv3.GlobalRoleBinding{{
		UserName:       "u-admin",
		GlobalRoleName: "admin",
	}}}, nil
}

type routeFailingBindings struct{}

func (routeFailingBindings) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	return nil, errors.New("bindings unavailable")
}

type routeRecoveryRoles struct{}

func (routeRecoveryRoles) Get(name string, _ metav1.GetOptions) (*apiv3.GlobalRole, error) {
	return &apiv3.GlobalRole{ObjectMeta: metav1.ObjectMeta{Name: name}, Builtin: true}, nil
}

type routeRecoveryTokenManager struct {
	tokens  map[string]*apiv3.Token
	deleted []string
	created int
}

func (m *routeRecoveryTokenManager) NewLoginToken(userID string, principal apiv3.Principal, _ []apiv3.Principal, _ string, _ int64, _ string) (*apiv3.Token, string, error) {
	m.created++
	return &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "effective"}, UserID: userID, UserPrincipal: principal}, "secret", nil
}

func (m *routeRecoveryTokenManager) UpdateToken(token *apiv3.Token) (*apiv3.Token, error) {
	return token, nil
}

func (m *routeRecoveryTokenManager) GetToken(value string) (accessor.TokenAccessor, int, error) {
	token, ok := m.tokens[value]
	if !ok {
		return nil, http.StatusNotFound, errors.New("token not found")
	}
	return token, http.StatusOK, nil
}

func (m *routeRecoveryTokenManager) GetTokenByName(name string) (accessor.TokenAccessor, int, error) {
	for _, token := range m.tokens {
		if token.Name == name {
			return token, 0, nil
		}
	}
	return nil, http.StatusNotFound, errors.New("token not found")
}

func (m *routeRecoveryTokenManager) GetTokenForRecovery(value string) (*apiv3.Token, bool, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return nil, false, errors.New("invalid token")
	}
	token, ok := m.tokens[value]
	if !ok || token.Name != parts[0] || token.Token != parts[1] {
		return nil, false, errors.New("invalid token secret")
	}
	return token, true, nil
}

func (m *routeRecoveryTokenManager) DeleteTokenByName(name string) (int, error) {
	m.deleted = append(m.deleted, name)
	return http.StatusOK, nil
}

func TestNewRootMuxRecoveryRouteRestoresThroughRegisteredHandler(t *testing.T) {
	t.Parallel()

	originalValue := "admin-token:admin-secret"
	effectiveValue := "effective-token:effective-secret"
	original := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-token"},
		UserID:     "u-admin",
		Token:      "admin-secret",
	}
	effective := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "effective-token"},
		UserID:     "u-target",
		Token:      "effective-secret",
	}
	tokens.BindImpersonationSession(effective, original)
	manager := &routeRecoveryTokenManager{tokens: map[string]*apiv3.Token{
		originalValue: original, effectiveValue: effective,
	}}
	handler := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter {
			return routeRecoveryUsers{"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}}}
		},
		func(string) impersonation.UserAttributeGetter { return routeRecoveryAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister { return routeRecoveryBindings{} },
		func(string) impersonation.GlobalRoleGetter { return routeRecoveryRoles{} },
		manager,
	)
	routes := testRoutes()
	routes.impersonation = handler

	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: originalValue})
	req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: effectiveValue})
	rec := httptest.NewRecorder()
	newRootMux(routes).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"restored":true`)
	require.Equal(t, []string{"effective-token"}, manager.deleted)

	regular := httptest.NewRecorder()
	newRootMux(routes).ServeHTTP(regular, httptest.NewRequest(http.MethodPost, "/v1/logout", nil))
	require.Equal(t, http.StatusUnauthorized, regular.Code)
}

func TestNewRootMuxDirectImpersonationRouteUsesCanonicalAdapter(t *testing.T) {
	t.Parallel()

	enabled := true
	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token"}, UserID: "u-admin"}
	manager := &routeRecoveryTokenManager{tokens: map[string]*apiv3.Token{"original": original}}
	handler := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter {
			return routeRecoveryUsers{
				"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
				"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
			}
		},
		func(string) impersonation.UserAttributeGetter { return routeRecoveryAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister { return routeRecoveryBindings{} },
		func(string) impersonation.GlobalRoleGetter { return routeRecoveryRoles{} },
		manager,
	)
	routes := testRoutes()
	routes.impersonation = handler

	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", strings.NewReader(`{"userId":"u-target"}`))
	req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
	req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{Name: "u-admin"}))
	rec := httptest.NewRecorder()

	newRootMux(routes).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, rec.Result().Cookies(), 2)
	assert.Contains(t, rec.Body.String(), `"impersonating":true`)
	assert.Equal(t, 1, manager.created)
}

func TestNewRootMuxDirectImpersonationRouteRejectsUnauthorizedTarget(t *testing.T) {
	t.Parallel()

	disabled := false
	manager := &routeRecoveryTokenManager{tokens: map[string]*apiv3.Token{
		"original": {ObjectMeta: metav1.ObjectMeta{Name: "admin-token"}, UserID: "u-admin"},
	}}
	handler := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter {
			return routeRecoveryUsers{
				"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
				"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}},
			}
		},
		func(string) impersonation.UserAttributeGetter { return routeRecoveryAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister { return routeRecoveryBindings{} },
		func(string) impersonation.GlobalRoleGetter { return routeRecoveryRoles{} },
		manager,
	)
	routes := testRoutes()
	routes.impersonation = handler
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", strings.NewReader(`{"userId":"u-target"}`))
	req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
	req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{Name: "u-admin"}))
	rec := httptest.NewRecorder()

	newRootMux(routes).ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Result().Cookies())
	assert.Equal(t, 0, manager.created)
}

func TestNewRootMuxDirectImpersonationRouteStatusMatrix(t *testing.T) {
	enabled := true
	disabled := false
	tests := []struct {
		name       string
		auth       bool
		target     *apiv3.User
		nested     bool
		failing    bool
		tokenUser  string
		wantStatus int
	}{
		{name: "unauthenticated", wantStatus: http.StatusUnauthorized},
		{name: "unauthorized target", auth: true, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}}, wantStatus: http.StatusForbidden},
		{name: "nested", auth: true, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, nested: true, wantStatus: http.StatusConflict},
		{name: "lookup failure", auth: true, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, failing: true, wantStatus: http.StatusInternalServerError},
		{name: "forged caller", auth: true, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, tokenUser: "u-other", wantStatus: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := routeRecoveryUsers{"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}}}
			if test.target != nil {
				users["u-target"] = test.target
			}
			tokenUser := test.tokenUser
			if tokenUser == "" {
				tokenUser = "u-admin"
			}
			manager := &routeRecoveryTokenManager{tokens: map[string]*apiv3.Token{
				"original": {ObjectMeta: metav1.ObjectMeta{Name: "admin-token"}, UserID: tokenUser},
			}}
			handler := impersonation.NewHandlerForTest(
				func(string) impersonation.UserGetter { return users },
				func(string) impersonation.UserAttributeGetter { return routeRecoveryAttributes{} },
				func(string) impersonation.GlobalRoleBindingLister {
					if test.failing {
						return routeFailingBindings{}
					}
					return routeRecoveryBindings{}
				},
				func(string) impersonation.GlobalRoleGetter { return routeRecoveryRoles{} },
				manager,
			)
			routes := testRoutes()
			routes.impersonation = handler
			req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", strings.NewReader(`{"userId":"u-target"}`))
			req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
			if test.nested {
				req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "nested"})
			}
			if test.auth {
				req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{Name: "u-admin"}))
			}
			rec := httptest.NewRecorder()

			newRootMux(routes).ServeHTTP(rec, req)

			require.Equal(t, test.wantStatus, rec.Code)
			require.Empty(t, rec.Result().Cookies())
			assert.Equal(t, 0, manager.created)
		})
	}
}

func TestNewRootMuxDirectImpersonationRouteRejectsMalformedBody(t *testing.T) {
	t.Parallel()

	handler, manager := newRegisteredImpersonationHandler()
	routes := testRoutes()
	routes.impersonation = handler
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", strings.NewReader(`{"userId":`))
	req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{Name: "u-admin"}))
	rec := httptest.NewRecorder()

	newRootMux(routes).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, rec.Result().Cookies())
	assert.Equal(t, 0, manager.created)
}

func newRegisteredImpersonationHandler() (*impersonation.Handler, *routeRecoveryTokenManager) {
	originalValue := "admin-token:admin-secret"
	effectiveValue := "effective-token:effective-secret"
	original := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-token"},
		UserID:     "u-admin",
		Token:      "admin-secret",
	}
	effective := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "effective-token"},
		UserID:     "u-target",
		Token:      "effective-secret",
	}
	tokens.BindImpersonationSession(effective, original)
	manager := &routeRecoveryTokenManager{tokens: map[string]*apiv3.Token{
		originalValue: original, effectiveValue: effective,
	}}
	handler := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter {
			return routeRecoveryUsers{"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}}}
		},
		func(string) impersonation.UserAttributeGetter { return routeRecoveryAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister { return routeRecoveryBindings{} },
		func(string) impersonation.GlobalRoleGetter { return routeRecoveryRoles{} },
		manager,
	)
	return handler, manager
}

func TestRegisteredAuthenticatedRouteAuditsImpersonationAndIsolatesContext(t *testing.T) {
	t.Parallel()

	handler, _ := newRegisteredImpersonationHandler()
	routes := testRoutes()
	routes.impersonation = handler
	root := newRootMux(routes)
	output := &bytes.Buffer{}
	writer, err := audit.NewWriter(output, audit.WriterOptions{})
	require.NoError(t, err)
	audited := audit.NewAuditLogMiddleware(writer, audit.WithIncludeUsername(func() bool { return true }))(handler.AuditMiddleware(root))

	first := httptest.NewRequest(http.MethodGet, "/v3/impersonation-sessions/current", nil)
	first.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "admin-token:admin-secret"})
	first.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "effective-token:effective-secret"})
	first = first.WithContext(request.WithUser(first.Context(), &user.DefaultInfo{
		Name: "u-target",
		Extra: map[string][]string{
			tokens.ImpersonationTokenIDUserExtra: {"effective-token"},
		},
	}))
	firstResponse := httptest.NewRecorder()
	audited.ServeHTTP(firstResponse, first)
	require.Equal(t, http.StatusOK, firstResponse.Code)

	second := httptest.NewRequest(http.MethodGet, "/v3/impersonation-sessions/current", nil)
	second = second.WithContext(request.WithUser(second.Context(), &user.DefaultInfo{Name: "u-target"}))
	secondResponse := httptest.NewRecorder()
	audited.ServeHTTP(secondResponse, second)
	require.Equal(t, http.StatusNotFound, secondResponse.Code)

	entries := strings.Split(strings.TrimSpace(output.String()), "\n")
	require.Len(t, entries, 2)
	assert.Contains(t, entries[0], `"name":"u-target"`)
	assert.Contains(t, entries[0], `"impersonatedBy"`)
	assert.Contains(t, entries[0], `"name":"u-admin"`)
	assert.Contains(t, entries[0], `"sessionId":"admin-token"`)
	assert.NotContains(t, entries[0], "admin-secret")
	assert.NotContains(t, entries[1], `"impersonatedBy"`)
	assert.NotContains(t, entries[1], `"sessionId"`)
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
