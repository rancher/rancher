package impersonation

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/tokens"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type testUsers map[string]*apiv3.User

func (u testUsers) Get(name string, _ metav1.GetOptions) (*apiv3.User, error) {
	user, ok := u[name]
	if !ok {
		return nil, apierrors.NewNotFound(apiv3.Resource("user"), name)
	}
	return user, nil
}

type testUserAttributes map[string]*apiv3.UserAttribute

func (a testUserAttributes) Get(name string, _ metav1.GetOptions) (*apiv3.UserAttribute, error) {
	attributes, ok := a[name]
	if !ok {
		return nil, apierrors.NewNotFound(apiv3.Resource("userattribute"), name)
	}
	return attributes, nil
}

type testBindings struct {
	items []apiv3.GlobalRoleBinding
}

func (b testBindings) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	return &apiv3.GlobalRoleBindingList{Items: b.items}, nil
}

type testRoles map[string]*apiv3.GlobalRole

func (r testRoles) Get(name string, _ metav1.GetOptions) (*apiv3.GlobalRole, error) {
	role, ok := r[name]
	if !ok {
		return nil, apierrors.NewNotFound(apiv3.Resource("globalrole"), name)
	}
	return role, nil
}

type failingBindings struct{ err error }

func (f failingBindings) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	return nil, f.err
}

type failingUsers struct{ err error }

func (f failingUsers) Get(string, metav1.GetOptions) (*apiv3.User, error) {
	return nil, f.err
}

type failingAttributes struct{ err error }

func (f failingAttributes) Get(string, metav1.GetOptions) (*apiv3.UserAttribute, error) {
	return nil, f.err
}

type failingRoles struct{ err error }

func (f failingRoles) Get(string, metav1.GetOptions) (*apiv3.GlobalRole, error) {
	return nil, f.err
}

type roleGetterFunc func(string, metav1.GetOptions) (*apiv3.GlobalRole, error)

func (f roleGetterFunc) Get(name string, options metav1.GetOptions) (*apiv3.GlobalRole, error) {
	return f(name, options)
}

type testTokenManager struct {
	token           *apiv3.Token
	key             string
	tokens          map[string]accessor.TokenAccessor
	errors          map[string]error
	deleted         []string
	ttl             int64
	createErr       error
	created         int
	updateErr       error
	recoveryExpired bool
	recoveryErr     error
}

func (m *testTokenManager) NewLoginToken(userID string, principal apiv3.Principal, _ []apiv3.Principal, _ string, ttl int64, _ string) (*apiv3.Token, string, error) {
	m.ttl = ttl
	m.created++
	if m.createErr != nil {
		return nil, "", m.createErr
	}
	if m.token == nil {
		m.token = &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "impersonation-token"}, UserID: userID, UserPrincipal: principal}
	}
	if m.key == "" {
		m.key = "effective-key"
	}
	return m.token, m.key, nil
}

func (m *testTokenManager) UpdateToken(token *apiv3.Token) (*apiv3.Token, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	m.token = token
	return token, nil
}

func (m *testTokenManager) GetTokenForRecovery(value string) (*apiv3.Token, bool, error) {
	if m.recoveryErr != nil {
		return nil, false, m.recoveryErr
	}
	token, ok := m.tokens[value].(*apiv3.Token)
	if !ok {
		return nil, false, errors.New("token not found")
	}
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || token.Name != parts[0] || (token.Token != "" && token.Token != parts[1]) {
		return nil, false, errors.New("invalid token secret")
	}
	return token, m.recoveryExpired, nil
}

func (m *testTokenManager) GetToken(value string) (accessor.TokenAccessor, int, error) {
	if err, ok := m.errors[value]; ok {
		return nil, http.StatusUnauthorized, err
	}
	if token, ok := m.tokens[value]; ok {
		return token, http.StatusOK, nil
	}
	return nil, http.StatusNotFound, errors.New("token not found")
}

func (m *testTokenManager) DeleteTokenByName(name string) (int, error) {
	m.deleted = append(m.deleted, name)
	return http.StatusOK, nil
}

var _ tokenManager = (*testTokenManager)(nil)

func TestIsAdminRecognizesCustomFullAccessRole(t *testing.T) {
	t.Parallel()

	h := &Handler{
		users:          func(string) userGetter { return testUsers{"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}}} },
		userAttributes: func(string) userAttributeGetter { return testUserAttributes{} },
		globalRoleBindings: func(string) globalRoleBindingLister {
			return testBindings{items: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "custom-admin"}}}
		},
		globalRoles: func(string) globalRoleGetter {
			return testRoles{"custom-admin": {
				ObjectMeta: metav1.ObjectMeta{Name: "custom-admin"},
				Rules: []rbacv1.PolicyRule{
					{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
					{NonResourceURLs: []string{"*"}, Verbs: []string{"*"}},
				},
			}}
		},
	}

	admin, err := h.isAdmin("u-admin")
	if err != nil || !admin {
		t.Fatal("expected custom full-access role to grant administrator access")
	}
}

func TestCanImpersonateUsesSharedEligibilityDecision(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false
	tests := []struct {
		name        string
		caller      string
		target      *apiv3.User
		bindings    []apiv3.GlobalRoleBinding
		wantAllowed bool
	}{
		{
			name:        "administrator and valid target",
			caller:      "u-admin",
			target:      &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
			bindings:    []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "admin"}},
			wantAllowed: true,
		},
		{
			name:     "non administrator caller",
			caller:   "u-user",
			target:   &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
			bindings: nil,
		},
		{
			name:     "disabled target",
			caller:   "u-admin",
			target:   &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}},
			bindings: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "admin"}},
		},
		{
			name:     "system target",
			caller:   "u-admin",
			target:   &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"system:serviceaccount"}},
			bindings: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "admin"}},
		},
		{
			name:     "target without principal",
			caller:   "u-admin",
			target:   &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled},
			bindings: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "admin"}},
		},
		{
			name:   "administrator target",
			caller: "u-admin",
			target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
			bindings: []apiv3.GlobalRoleBinding{
				{UserName: "u-admin", GlobalRoleName: "admin"},
				{UserName: "u-target", GlobalRoleName: "admin"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			users := testUsers{
				"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
				"u-user":   {ObjectMeta: metav1.ObjectMeta{Name: "u-user"}},
				"u-target": test.target,
			}
			h := &Handler{
				users:          func(string) userGetter { return users },
				userAttributes: func(string) userAttributeGetter { return testUserAttributes{} },
				globalRoleBindings: func(string) globalRoleBindingLister {
					return testBindings{items: test.bindings}
				},
				globalRoles: func(string) globalRoleGetter {
					return testRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
				},
			}

			allowed, err := h.CanImpersonate(test.caller, "u-target")
			if err != nil || allowed != test.wantAllowed {
				t.Fatalf("unexpected eligibility: allowed=%v err=%v, want %v", allowed, err, test.wantAllowed)
			}
		})
	}
}

func TestIsAdminRecognizesBuiltinAdminRole(t *testing.T) {
	t.Parallel()

	h := lifecycleHandler(&testTokenManager{})
	admin, err := h.isAdmin("u-admin")
	if err != nil || !admin {
		t.Fatalf("expected builtin admin role to grant admin, admin=%v err=%v", admin, err)
	}
}

func TestIsAdminRejectsIncompleteCustomRole(t *testing.T) {
	t.Parallel()

	h := lifecycleHandler(&testTokenManager{})
	h.globalRoleBindings = func(string) globalRoleBindingLister {
		return testBindings{items: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "partial-admin"}}}
	}
	h.globalRoles = func(string) globalRoleGetter {
		return testRoles{"partial-admin": {
			ObjectMeta: metav1.ObjectMeta{Name: "partial-admin"},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}},
		}}
	}

	admin, err := h.isAdmin("u-admin")
	if err != nil || admin {
		t.Fatalf("expected incomplete custom role not to grant admin, admin=%v err=%v", admin, err)
	}
}

func TestIsAdminRecognizesUserNameBinding(t *testing.T) {
	t.Parallel()

	h := lifecycleHandler(&testTokenManager{})
	admin, err := h.isAdmin("u-admin")
	if err != nil || !admin {
		t.Fatalf("expected UserName binding to grant admin, admin=%v err=%v", admin, err)
	}
}

func TestIsAdminTreatsMissingUserAttributesAsNoGroups(t *testing.T) {
	t.Parallel()

	h := lifecycleHandler(&testTokenManager{})
	h.globalRoleBindings = func(string) globalRoleBindingLister {
		return testBindings{items: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "admin"}}}
	}
	admin, err := h.isAdmin("u-admin")
	if err != nil || !admin {
		t.Fatalf("expected missing UserAttribute to be ignored, admin=%v err=%v", admin, err)
	}
}

func TestIsAdminRecognizesPrincipalBinding(t *testing.T) {
	t.Parallel()

	h := &Handler{
		users: func(string) userGetter {
			return testUsers{"u-admin": {
				ObjectMeta:   metav1.ObjectMeta{Name: "u-admin"},
				PrincipalIDs: []string{"local://user"},
			}}
		},
		userAttributes: func(string) userAttributeGetter { return testUserAttributes{} },
		globalRoleBindings: func(string) globalRoleBindingLister {
			return testBindings{items: []apiv3.GlobalRoleBinding{{UserPrincipalName: "local://user", GlobalRoleName: "admin"}}}
		},
		globalRoles: func(string) globalRoleGetter {
			return testRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
	}

	admin, err := h.isAdmin("u-admin")
	if err != nil || !admin {
		t.Fatal("expected administrator principal binding to grant administrator access")
	}
}

func TestIsAdminRecognizesGroupBinding(t *testing.T) {
	t.Parallel()

	h := &Handler{
		users: func(string) userGetter {
			return testUsers{"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}}}
		},
		userAttributes: func(string) userAttributeGetter {
			return testUserAttributes{"u-admin": {
				GroupPrincipals: map[string]apiv3.Principals{
					"local": {Items: []apiv3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: "local_group://admins"}}}},
				},
			}}
		},
		globalRoleBindings: func(string) globalRoleBindingLister {
			return testBindings{items: []apiv3.GlobalRoleBinding{{GroupPrincipalName: "local_group://admins", GlobalRoleName: "admin"}}}
		},
		globalRoles: func(string) globalRoleGetter {
			return testRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
	}

	admin, err := h.isAdmin("u-admin")
	if err != nil || !admin {
		t.Fatal("expected group administrator binding to grant administrator access")
	}
}

func TestIsAdminIgnoresEmptyPrincipalSelectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		principals []string
		groups     []apiv3.Principal
		binding    apiv3.GlobalRoleBinding
	}{
		{
			name:       "empty user principal",
			principals: []string{""},
			binding:    apiv3.GlobalRoleBinding{UserPrincipalName: "", GlobalRoleName: "admin"},
		},
		{
			name:    "empty group principal",
			groups:  []apiv3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: ""}}},
			binding: apiv3.GlobalRoleBinding{GroupPrincipalName: "", GlobalRoleName: "admin"},
		},
		{
			name:    "entirely empty binding",
			binding: apiv3.GlobalRoleBinding{GlobalRoleName: "admin"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			h := &Handler{
				users: func(string) userGetter {
					return testUsers{"u-user": {
						ObjectMeta:   metav1.ObjectMeta{Name: "u-user"},
						PrincipalIDs: test.principals,
					}}
				},
				userAttributes: func(string) userAttributeGetter {
					return testUserAttributes{"u-user": {
						GroupPrincipals: map[string]apiv3.Principals{
							"local": {Items: test.groups},
						},
					}}
				},
				globalRoleBindings: func(string) globalRoleBindingLister {
					return testBindings{items: []apiv3.GlobalRoleBinding{test.binding}}
				},
				globalRoles: func(string) globalRoleGetter {
					return testRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
				},
			}

			admin, err := h.isAdmin("u-user")
			if err != nil || admin {
				t.Fatalf("expected empty selectors not to grant admin, admin=%v err=%v", admin, err)
			}
		})
	}
}

func TestIsAdminDoesNotTreatBuiltinUserRoleAsAdmin(t *testing.T) {
	t.Parallel()

	h := lifecycleHandler(&testTokenManager{})
	h.globalRoleBindings = func(string) globalRoleBindingLister {
		return testBindings{items: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "user"}}}
	}
	h.globalRoles = func(string) globalRoleGetter {
		return testRoles{"user": {ObjectMeta: metav1.ObjectMeta{Name: "user"}, Builtin: true}}
	}

	admin, err := h.isAdmin("u-admin")
	if err != nil || admin {
		t.Fatalf("expected builtin user role not to grant admin, admin=%v err=%v", admin, err)
	}
}

func TestIsAdminPropagatesLookupErrors(t *testing.T) {
	t.Parallel()

	errLookup := errors.New("lookup failed")
	tests := []struct {
		name string
		set  func(*Handler)
	}{
		{
			name: "bindings",
			set: func(h *Handler) {
				h.globalRoleBindings = func(string) globalRoleBindingLister { return failingBindings{err: errLookup} }
			},
		},
		{
			name: "user",
			set: func(h *Handler) {
				h.users = func(string) userGetter { return failingUsers{err: errLookup} }
			},
		},
		{
			name: "attributes",
			set: func(h *Handler) {
				h.userAttributes = func(string) userAttributeGetter { return failingAttributes{err: errLookup} }
			},
		},
		{
			name: "role",
			set: func(h *Handler) {
				h.globalRoleBindings = func(string) globalRoleBindingLister {
					return testBindings{items: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "admin"}}}
				}
				h.globalRoles = func(string) globalRoleGetter { return failingRoles{err: errLookup} }
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := lifecycleHandler(&testTokenManager{})
			test.set(h)

			admin, err := h.isAdmin("u-admin")
			if err == nil || admin {
				t.Fatalf("expected lookup error, admin=%v err=%v", admin, err)
			}
		})
	}
}

func TestStartReturnsInternalErrorForAuthorizationLookupFailure(t *testing.T) {
	t.Parallel()

	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token": &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
	}}
	h := lifecycleHandler(manager)
	h.globalRoleBindings = func(string) globalRoleBindingLister {
		return failingBindings{err: errors.New("bindings unavailable")}
	}
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))
	response := httptest.NewRecorder()

	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected authorization lookup failure to return 500, got %d", response.Code)
	}
	if manager.created != 0 || len(response.Result().Cookies()) != 0 {
		t.Fatalf("expected no token or cookies after authorization failure, created=%d cookies=%v", manager.created, response.Result().Cookies())
	}
}

func TestStartReturnsInternalErrorForTargetAdminLookupFailure(t *testing.T) {
	t.Parallel()

	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token": &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
	}}
	h := lifecycleHandler(manager)
	h.globalRoleBindings = func(string) globalRoleBindingLister {
		return testBindings{items: []apiv3.GlobalRoleBinding{
			{UserName: "u-admin", GlobalRoleName: "admin"},
			{UserName: "u-target", GlobalRoleName: "target-admin"},
		}}
	}
	h.globalRoles = func(string) globalRoleGetter {
		return roleGetterFunc(func(name string, _ metav1.GetOptions) (*apiv3.GlobalRole, error) {
			if name == "target-admin" {
				return nil, errors.New("target role unavailable")
			}
			return &apiv3.GlobalRole{ObjectMeta: metav1.ObjectMeta{Name: name}, Builtin: true}, nil
		})
	}
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))
	response := httptest.NewRecorder()

	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected target authorization lookup failure to return 500, got %d", response.Code)
	}
	if manager.created != 0 || len(response.Result().Cookies()) != 0 {
		t.Fatalf("expected no token or cookies after target authorization failure, created=%d cookies=%v", manager.created, response.Result().Cookies())
	}
}

func TestStartRejectsNonAdminCaller(t *testing.T) {
	t.Parallel()

	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"caller-token": &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "caller-token-id"}, UserID: "u-admin"},
	}}
	h := lifecycleHandler(manager)
	h.globalRoleBindings = func(string) globalRoleBindingLister {
		return testBindings{items: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "user"}}}
	}
	h.globalRoles = func(string) globalRoleGetter {
		return testRoles{"user": {ObjectMeta: metav1.ObjectMeta{Name: "user"}, Builtin: true}}
	}
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "caller-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))
	response := httptest.NewRecorder()

	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin caller to return 403, got %d", response.Code)
	}
	if manager.created != 0 || len(response.Result().Cookies()) != 0 {
		t.Fatalf("expected no token or cookies for non-admin caller, created=%d cookies=%v", manager.created, response.Result().Cookies())
	}
}

func TestStartRejectsAdminTarget(t *testing.T) {
	t.Parallel()

	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token": &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
	}}
	h := lifecycleHandler(manager)
	h.globalRoleBindings = func(string) globalRoleBindingLister {
		return testBindings{items: []apiv3.GlobalRoleBinding{
			{UserName: "u-admin", GlobalRoleName: "admin"},
			{UserName: "u-target", GlobalRoleName: "admin"},
		}}
	}
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))
	response := httptest.NewRecorder()

	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected admin target to return 403, got %d", response.Code)
	}
	if manager.created != 0 || len(response.Result().Cookies()) != 0 {
		t.Fatalf("expected no token or cookies for admin target, created=%d cookies=%v", manager.created, response.Result().Cookies())
	}
}

func TestStartSessionRejectsInvalidTargetsWithoutCreatingTokens(t *testing.T) {
	enabled := true
	disabled := false
	tests := []struct {
		name       string
		target     *apiv3.User
		targetRole bool
	}{
		{name: "missing target", target: nil},
		{name: "disabled target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}}},
		{name: "system target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"system://target"}}},
		{name: "administrator target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, targetRole: true},
		{name: "target without principal", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled}},
		{name: "invalid principal", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"invalid-principal"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
				"admin-token": &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
			}}
			h := lifecycleHandler(manager)
			users := testUsers{
				"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
			}
			if test.target != nil {
				users["u-target"] = test.target
			}
			h.users = func(string) userGetter { return users }
			if test.targetRole {
				h.globalRoleBindings = func(string) globalRoleBindingLister {
					return testBindings{items: []apiv3.GlobalRoleBinding{
						{UserName: "u-admin", GlobalRoleName: "admin"},
						{UserName: "u-target", GlobalRoleName: "admin"},
					}}
				}
			}
			req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
			req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))

			result, err := h.StartSession(req, "u-target")

			if result != nil || err == nil {
				t.Fatalf("expected target rejection, result=%#v err=%v", result, err)
			}
			startErr, ok := err.(*StartError)
			if !ok || startErr.Status != http.StatusForbidden {
				t.Fatalf("expected 403 target rejection, got %T %#v", err, err)
			}
			if manager.created != 0 {
				t.Fatalf("expected no token creation, got %d", manager.created)
			}
		})
	}
}

func TestStartSessionSuccessCreatesExactlyOneBoundToken(t *testing.T) {
	tests := []struct {
		name       string
		targetID   string
		targetName string
		principal  string
	}{
		{name: "standard target", targetID: "u-target", targetName: "u-target", principal: "local://target"},
		{name: "target with display name", targetID: "u-target-display", targetName: "u-target-display", principal: "local://display"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}
			manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{"admin-token": original}}
			h := lifecycleHandler(manager)
			h.users = func(string) userGetter {
				return testUsers{
					"u-admin":     {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
					test.targetID: {ObjectMeta: metav1.ObjectMeta{Name: test.targetName}, PrincipalIDs: []string{test.principal}},
				}
			}
			req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
			req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))

			result, err := h.StartSession(req, test.targetID)

			if err != nil {
				t.Fatalf("expected success, got %v", err)
			}
			if result == nil || result.UserID != test.targetName || result.PrincipalID != test.principal {
				t.Fatalf("unexpected start response: %#v", result)
			}
			if manager.created != 1 {
				t.Fatalf("expected exactly one created token, got %d", manager.created)
			}
			if !tokens.IsImpersonationSessionBound(manager.token, original) {
				t.Fatalf("expected effective token to be bound to original token: %#v", manager.token.GetAnnotations())
			}
			annotations := manager.token.GetAnnotations()
			if annotations[tokens.ImpersonatorTokenAnnotation] != original.Name ||
				annotations[tokens.ImpersonatorUserAnnotation] != "u-admin" ||
				annotations[tokens.ImpersonationSessionAnnotation] != "true" {
				t.Fatalf("unexpected token binding: %#v", annotations)
			}
		})
	}
}

func TestStartCallerUserLookupFailureReturns500WithoutTokenOrCookies(t *testing.T) {
	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token": &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
	}}
	h := lifecycleHandler(manager)
	h.users = func(string) userGetter {
		return failingUsers{err: errors.New("caller user lookup failed")}
	}
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))
	response := httptest.NewRecorder()

	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected caller lookup failure to return 500, got %d", response.Code)
	}
	if manager.created != 0 || len(response.Result().Cookies()) != 0 {
		t.Fatalf("expected no token or cookies after caller lookup failure, created=%d cookies=%v", manager.created, response.Result().Cookies())
	}
}

func TestStartSessionReturnsInternalErrorWhenTokenCreationFails(t *testing.T) {
	manager := &testTokenManager{
		tokens:    map[string]accessor.TokenAccessor{"admin-token": &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}},
		createErr: errors.New("create failed"),
	}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))

	result, err := h.StartSession(req, "u-target")

	if result != nil || err == nil {
		t.Fatalf("expected token creation error, result=%#v err=%v", result, err)
	}
	startErr, ok := err.(*StartError)
	if !ok || startErr.Status != http.StatusInternalServerError {
		t.Fatalf("expected 500 token creation error, got %T %#v", err, err)
	}
	if manager.created != 1 || len(manager.deleted) != 0 {
		t.Fatalf("unexpected token lifecycle after create failure: created=%d deleted=%v", manager.created, manager.deleted)
	}
}

func TestStartSessionRejectsInvalidOriginalSessions(t *testing.T) {
	expired := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "expired-token",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute)),
		},
		UserID:    "u-admin",
		TTLMillis: 1,
	}
	tests := []struct {
		name          string
		cookie        string
		originalToken accessor.TokenAccessor
		wantStatus    int
	}{
		{name: "missing cookie", wantStatus: http.StatusUnauthorized},
		{name: "missing token", cookie: "missing-token", wantStatus: http.StatusUnauthorized},
		{name: "foreign token", cookie: "foreign-token", originalToken: &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "foreign-token"}, UserID: "u-other"}, wantStatus: http.StatusUnauthorized},
		{name: "expired token", cookie: "expired-token", originalToken: expired, wantStatus: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{}}
			if test.originalToken != nil {
				manager.tokens[test.cookie] = test.originalToken
			}
			h := lifecycleHandler(manager)
			req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
			if test.cookie != "" {
				req.AddCookie(&http.Cookie{Name: sessionCookie, Value: test.cookie})
			}
			req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))

			result, err := h.StartSession(req, "u-target")

			if result != nil || err == nil {
				t.Fatalf("expected invalid original session, result=%#v err=%v", result, err)
			}
			startErr, ok := err.(*StartError)
			if !ok || startErr.Status != test.wantStatus {
				t.Fatalf("expected %d, got %T %#v", test.wantStatus, err, err)
			}
			if manager.created != 0 {
				t.Fatalf("expected no token creation, got %d", manager.created)
			}
		})
	}
}

func TestRecoverRestoresOriginalAfterExpiredEffectiveToken(t *testing.T) {
	t.Parallel()

	originalValue := "admin-token:admin-secret"
	effectiveValue := "effective-token:effective-secret"
	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token"}, UserID: "u-admin", Token: "admin-secret"}
	effective := boundEffectiveToken("effective-token-id", "u-target", original)
	effective.Token = "effective-secret"
	effective.Name = "effective-token"
	manager := &testTokenManager{
		tokens: map[string]accessor.TokenAccessor{
			originalValue:  original,
			effectiveValue: effective,
		},
		recoveryExpired: true,
	}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: originalValue})
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: effectiveValue})
	response := httptest.NewRecorder()

	h.Recover(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("expected recovery to return 200, got %d", response.Code)
	}
	if len(manager.deleted) != 1 || manager.deleted[0] != "effective-token" {
		t.Fatalf("expected effective token revocation, got %#v", manager.deleted)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"restored":true`) {
		t.Fatalf("expected restored response, got %s", body)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Name != sessionCookie || cookies[0].Value != originalValue || cookies[1].Name != tokens.OriginalSessionCookie || cookies[1].MaxAge != -1 {
		t.Fatalf("unexpected recovery cookies: %#v", cookies)
	}
}

func TestRecoverCleansUpMissingOrMismatchedEffectiveToken(t *testing.T) {
	t.Parallel()

	originalValue := "admin-token:admin-secret"
	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token"}, UserID: "u-admin", Token: "admin-secret"}
	otherOriginal := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "other-admin-id"}, UserID: "u-other-admin"}
	tests := []struct {
		name      string
		effective accessor.TokenAccessor
	}{
		{name: "missing", effective: nil},
		{name: "mismatched", effective: boundEffectiveToken("effective-token-id", "u-target", otherOriginal)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokensByValue := map[string]accessor.TokenAccessor{originalValue: original}
			if test.effective != nil {
				test.effective.(*apiv3.Token).Name = "effective-token"
				test.effective.(*apiv3.Token).Token = "effective-secret"
				tokensByValue["effective-token:effective-secret"] = test.effective
			}
			manager := &testTokenManager{tokens: tokensByValue}
			h := lifecycleHandler(manager)
			req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
			req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: originalValue})
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "effective-token:effective-secret"})
			response := httptest.NewRecorder()

			h.Recover(response, req)

			if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"restored":true`) {
				t.Fatalf("expected safe cleanup response, status=%d body=%s", response.Code, response.Body.String())
			}
			if len(manager.deleted) != 0 {
				t.Fatalf("expected no token deletion, got %#v", manager.deleted)
			}
		})
	}
}

func TestRecoverRejectsWrongSecretAndExpiredOriginal(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token"}, UserID: "u-admin", Token: "admin-secret"}
	effective := boundEffectiveToken("effective-token-id", "u-target", original)
	effective.Name = "effective-token"
	effective.Token = "effective-secret"
	manager := &testTokenManager{
		tokens: map[string]accessor.TokenAccessor{
			"admin-token:admin-secret":         original,
			"effective-token:effective-secret": effective,
		},
		recoveryExpired: true,
	}
	h := lifecycleHandler(manager)
	for _, test := range []struct {
		name           string
		originalValue  string
		effectiveValue string
	}{
		{name: "wrong effective secret", originalValue: "admin-token:admin-secret", effectiveValue: "effective-token:wrong-secret"},
		{name: "missing original", originalValue: "admin-token:wrong-secret", effectiveValue: "effective-token:effective-secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
			req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: test.originalValue})
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: test.effectiveValue})
			response := httptest.NewRecorder()

			h.Recover(response, req)

			if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"restored":true`) {
				t.Fatalf("expected safe cleanup response, status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if len(manager.deleted) != 0 {
		t.Fatalf("expected no token deletion for invalid secrets, got %#v", manager.deleted)
	}
}

func TestRecoverDoesNotRestoreExpiredOriginal(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "admin-token",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute)),
		},
		UserID:    "u-admin",
		Token:     "admin-secret",
		TTLMillis: 1,
	}
	effective := boundEffectiveToken("effective-token-id", "u-target", original)
	effective.Name = "effective-token"
	effective.Token = "effective-secret"
	manager := &testTokenManager{
		tokens: map[string]accessor.TokenAccessor{
			"admin-token:admin-secret":         original,
			"effective-token:effective-secret": effective,
		},
		recoveryExpired: true,
	}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "admin-token:admin-secret"})
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "effective-token:effective-secret"})
	response := httptest.NewRecorder()

	h.Recover(response, req)

	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"restored":true`) {
		t.Fatalf("expected expired original not to be restored, status=%d body=%s", response.Code, response.Body.String())
	}
	if len(manager.deleted) != 0 {
		t.Fatalf("expected no deletion for expired original, got %#v", manager.deleted)
	}
}

func TestRecoverIsIdempotentAfterRestoration(t *testing.T) {
	t.Parallel()

	originalValue := "admin-token:admin-secret"
	effectiveValue := "effective-token:effective-secret"
	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token"}, UserID: "u-admin", Token: "admin-secret"}
	effective := boundEffectiveToken("effective-token", "u-target", original)
	effective.Token = "effective-secret"
	manager := &testTokenManager{
		tokens:          map[string]accessor.TokenAccessor{originalValue: original, effectiveValue: effective},
		recoveryExpired: true,
	}
	h := lifecycleHandler(manager)

	first := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
	firstRequest.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: originalValue})
	firstRequest.AddCookie(&http.Cookie{Name: sessionCookie, Value: effectiveValue})
	h.Recover(first, firstRequest)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"restored":true`) {
		t.Fatalf("expected first recovery to restore original, status=%d body=%s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
	secondRequest.AddCookie(&http.Cookie{Name: sessionCookie, Value: originalValue})
	h.Recover(second, secondRequest)
	if second.Code != http.StatusOK || strings.Contains(second.Body.String(), `"restored":true`) {
		t.Fatalf("expected second recovery to be idempotent cleanup, status=%d body=%s", second.Code, second.Body.String())
	}
	if len(manager.deleted) != 1 {
		t.Fatalf("expected effective token to be revoked once, got %#v", manager.deleted)
	}
}

func TestRecoverRejectsNonImpersonationToken(t *testing.T) {
	t.Parallel()

	originalValue := "admin-token:admin-secret"
	effectiveValue := "regular-token:regular-secret"
	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token"}, UserID: "u-admin", Token: "admin-secret"}
	regular := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "regular-token"}, UserID: "u-target", Token: "regular-secret"}
	manager := &testTokenManager{
		tokens:          map[string]accessor.TokenAccessor{originalValue: original, effectiveValue: regular},
		recoveryExpired: true,
	}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: originalValue})
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: effectiveValue})
	response := httptest.NewRecorder()

	h.Recover(response, req)

	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"restored":true`) {
		t.Fatalf("expected regular token to be rejected safely, status=%d body=%s", response.Code, response.Body.String())
	}
	if len(manager.deleted) != 0 {
		t.Fatalf("expected regular token not to be deleted, got %#v", manager.deleted)
	}
}

func TestStartRejectsNestedImpersonation(t *testing.T) {
	t.Parallel()

	h := lifecycleHandler(&testTokenManager{})
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "already-impersonating"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))

	response := httptest.NewRecorder()
	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusConflict {
		t.Fatalf("expected nested impersonation to return 409, got %d", response.Code)
	}
}

func TestStartPathsShareCanonicalLifecycle(t *testing.T) {
	t.Parallel()

	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token": &apiv3.Token{UserID: "u-admin"},
	}}
	h := lifecycleHandler(manager)

	directRequest := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", strings.NewReader(`{"userId":"u-target"}`))
	directRequest.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	directRequest = directRequest.WithContext(request.WithUser(directRequest.Context(), &requestUser{Name: "u-admin"}))
	directResponse := httptest.NewRecorder()
	h.Start(directResponse, directRequest)

	actionRequest := httptest.NewRequest(http.MethodPost, "/v1/management.cattle.io.users/u-target?action=impersonate", nil)
	actionRequest.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	actionRequest = actionRequest.WithContext(request.WithUser(actionRequest.Context(), &requestUser{Name: "u-admin"}))
	actionResponse := httptest.NewRecorder()
	h.StartForUser(actionResponse, actionRequest, "u-target")

	if directResponse.Code != http.StatusOK || actionResponse.Code != http.StatusOK {
		t.Fatalf("expected both lifecycle entry points to succeed, got direct=%d action=%d", directResponse.Code, actionResponse.Code)
	}
	if manager.created != 2 {
		t.Fatalf("expected both entry points to delegate to the canonical start lifecycle, created %d tokens", manager.created)
	}
}

func TestStartLimitsImpersonationTTLToOriginalSession(t *testing.T) {
	t.Parallel()

	originalExpiryTime := time.Now().Add(2 * time.Minute)
	originalExpiry := originalExpiryTime.UTC().Format(time.RFC3339Nano)
	manager := &testTokenManager{
		tokens: map[string]accessor.TokenAccessor{
			"admin-token": &apiv3.Token{UserID: "u-admin", ExpiresAt: originalExpiry},
		},
	}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))

	response := httptest.NewRecorder()
	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusOK {
		t.Fatalf("expected impersonation to start, got %d", response.Code)
	}
	remaining := time.Until(originalExpiryTime)
	if manager.ttl <= 0 || time.Duration(manager.ttl)*time.Millisecond > remaining {
		t.Fatalf("expected TTL to be limited to original session, got %d", manager.ttl)
	}
}

func TestStartBindsSessionWithoutPersistingBearerSecret(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "token-admin"}, UserID: "u-admin"}
	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{"token-admin:admin-secret": original}}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token-admin:admin-secret"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))

	response := httptest.NewRecorder()
	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusOK {
		t.Fatalf("expected impersonation to start, got %d", response.Code)
	}
	annotations := manager.token.GetAnnotations()
	if annotations[tokens.ImpersonatorTokenAnnotation] != "token-admin" ||
		annotations[tokens.ImpersonatorUserAnnotation] != "u-admin" ||
		annotations[tokens.ImpersonationSessionAnnotation] != "true" {
		t.Fatalf("unexpected impersonation binding: %#v", annotations)
	}
	if !manager.token.UserPrincipal.Me {
		t.Fatal("effective impersonation principal must be marked as current")
	}
	for _, value := range annotations {
		if strings.Contains(value, "admin-secret") || strings.Contains(value, manager.key) {
			t.Fatal("impersonation metadata must not contain the bearer secret")
		}
	}
}

func TestStartCleansUpTokenWhenBindingFails(t *testing.T) {
	t.Parallel()

	manager := &testTokenManager{
		tokens:    map[string]accessor.TokenAccessor{"admin-token": &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}},
		updateErr: errors.New("update failed"),
	}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-admin"}))

	response := httptest.NewRecorder()
	h.StartForUser(response, req, "u-target")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected binding failure to return 500, got %d", response.Code)
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatalf("expected no cookies after binding failure, got %#v", response.Result().Cookies())
	}
	if len(manager.deleted) != 1 || manager.deleted[0] != "impersonation-token" {
		t.Fatalf("expected created token cleanup, got %#v", manager.deleted)
	}
}

func TestSecureCookiesRequireTLS(t *testing.T) {
	t.Parallel()

	h := &Handler{}

	forwardedRequest := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	forwardedRequest.Header.Set("X-Forwarded-Proto", "https")
	if h.isSecureRequest(forwardedRequest) {
		t.Fatal("expected an untrusted forwarded header not to enable Secure cookies")
	}

	tlsRequest := httptest.NewRequest(http.MethodGet, "https://localhost", nil)
	tlsRequest.TLS = &tls.ConnectionState{}
	if !h.isSecureRequest(tlsRequest) {
		t.Fatal("expected TLS requests to enable Secure cookies")
	}
}

func TestStopRevokesEffectiveTokenBeforeRestoringOriginal(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}
	effective := boundEffectiveToken("impersonation-token", "u-target", original)
	manager := &testTokenManager{
		token: effective,
		tokens: map[string]accessor.TokenAccessor{
			"admin-token":     original,
			"effective-token": effective,
		},
	}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodDelete, "/v3/impersonation-sessions/current", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "effective-token"})
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-target"}))

	response := httptest.NewRecorder()
	h.Stop(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("expected stop to return 200, got %d", response.Code)
	}
	if len(manager.deleted) != 1 || manager.deleted[0] != "impersonation-token" {
		t.Fatalf("expected effective token to be revoked, got %#v", manager.deleted)
	}
}

func TestStatusAcceptsBoundSession(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}
	effective := boundEffectiveToken("impersonation-token", "u-target", original)
	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token":     original,
		"effective-token": effective,
	}}
	h := lifecycleHandler(manager)
	req := impersonationRequest(http.MethodGet, "u-target")

	response := httptest.NewRecorder()
	h.Status(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("expected bound session status to return 200, got %d", response.Code)
	}

	var body responseBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected valid status response: %v", err)
	}
	if body.UserID != "u-target" || body.PrincipalID != "local://target" || !body.Impersonating {
		t.Fatalf("expected effective user identity in status response, got %#v", body)
	}
}

func TestStatusAndStopRejectUnrelatedSessions(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}
	otherOriginal := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "other-admin-token"}, UserID: "u-other-admin"}
	effective := boundEffectiveToken("impersonation-token", "u-target", otherOriginal)

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
			"admin-token":     original,
			"effective-token": effective,
		}}
		h := lifecycleHandler(manager)
		req := impersonationRequest(method, "u-target")
		response := httptest.NewRecorder()

		if method == http.MethodGet {
			h.Status(response, req)
		} else {
			h.Stop(response, req)
		}

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected unrelated %s session to return 401, got %d", method, response.Code)
		}
		if len(manager.deleted) != 0 {
			t.Fatalf("expected no deletion for unrelated %s session, got %#v", method, manager.deleted)
		}
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == sessionCookie && cookie.Value != "" {
				t.Fatalf("unexpected restored session cookie for unrelated %s session", method)
			}
		}
	}
}

func TestStatusRejectsUnmarkedEffectiveToken(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}
	effective := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "ordinary-token"}, UserID: "u-target"}
	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token":     original,
		"effective-token": effective,
	}}
	h := lifecycleHandler(manager)
	req := impersonationRequest(http.MethodGet, "u-target")
	response := httptest.NewRecorder()

	h.Status(response, req)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected an ordinary effective token to return 401, got %d", response.Code)
	}
}

func TestStopRejectsExpiredEffectiveToken(t *testing.T) {
	t.Parallel()

	manager := &testTokenManager{
		tokens: map[string]accessor.TokenAccessor{
			"admin-token": &apiv3.Token{UserID: "u-admin"},
		},
		errors: map[string]error{"effective-token": errors.New("token expired")},
	}
	h := lifecycleHandler(manager)
	req := httptest.NewRequest(http.MethodDelete, "/v3/impersonation-sessions/current", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "effective-token"})
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "admin-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-target"}))

	response := httptest.NewRecorder()
	h.Stop(response, req)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected expired effective token to return 401, got %d", response.Code)
	}
	if len(manager.deleted) != 0 {
		t.Fatalf("expected no token revocation after validation failure, got %#v", manager.deleted)
	}
}

func TestStatusRejectsMissingOrExpiredOriginalToken(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}
	effective := boundEffectiveToken("impersonation-token", "u-target", original)
	for _, test := range []struct {
		name   string
		tokens map[string]accessor.TokenAccessor
		errors map[string]error
	}{
		{
			name:   "missing",
			tokens: map[string]accessor.TokenAccessor{"effective-token": effective},
		},
		{
			name: "expired",
			tokens: map[string]accessor.TokenAccessor{
				"effective-token": effective,
			},
			errors: map[string]error{"admin-token": errors.New("token expired")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := &testTokenManager{tokens: test.tokens, errors: test.errors}
			h := lifecycleHandler(manager)
			req := impersonationRequest(http.MethodGet, "u-target")
			response := httptest.NewRecorder()

			h.Status(response, req)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected invalid original token to return 401, got %d", response.Code)
			}
			if len(manager.deleted) != 0 {
				t.Fatalf("expected no deletion, got %#v", manager.deleted)
			}
		})
	}
}

func TestAuditMiddlewareRecordsImpersonator(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"},
		UserID:     "u-admin",
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: "local://admin"},
			LoginName:  "admin",
			Provider:   "local",
		},
	}
	effective := boundEffectiveToken("impersonation-token", "u-target", original)
	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token":     original,
		"effective-token": effective,
	}}
	h := lifecycleHandler(manager)
	output := &bytes.Buffer{}
	writer, err := audit.NewWriter(output, audit.WriterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	handler := audit.NewAuditLogMiddleware(writer, audit.WithIncludeUsername(func() bool { return true }))(h.AuditMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))

	req := httptest.NewRequest(http.MethodGet, "/v3/management.cattle.io.users", nil)
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "admin-token"})
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "effective-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-target"}))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var entry struct {
		User struct {
			Name           string `json:"name"`
			ImpersonatedBy struct {
				Name      string `json:"name"`
				SessionID string `json:"sessionId"`
			} `json:"impersonatedBy"`
		} `json:"user"`
	}
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.User.Name != "u-target" || entry.User.ImpersonatedBy.Name != "u-admin" || entry.User.ImpersonatedBy.SessionID != "admin-token-id" {
		t.Fatalf("unexpected audit identity: %#v", entry.User)
	}
	if strings.Contains(output.String(), "admin-secret") {
		t.Fatalf("audit must not contain bearer values: %s", output.String())
	}
}

func TestAuditMiddlewareRecordsOpaqueAttributionWithoutUsernameFeature(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"},
		UserID:     "u-admin",
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: "local://admin"},
			LoginName:  "admin",
			Provider:   "local",
		},
	}
	effective := boundEffectiveToken("impersonation-token", "u-target", original)
	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token":     original,
		"effective-token": effective,
	}}
	h := lifecycleHandler(manager)
	output := &bytes.Buffer{}
	writer, err := audit.NewWriter(output, audit.WriterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	handler := audit.NewAuditLogMiddleware(writer, audit.WithIncludeUsername(func() bool { return false }))(h.AuditMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))

	req := impersonationRequest(http.MethodGet, "u-target")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("expected valid session to continue, got %d", response.Code)
	}
	var entry struct {
		User struct {
			Name           string `json:"name"`
			ImpersonatedBy struct {
				Name      string         `json:"name"`
				SessionID string         `json:"sessionId"`
				Identity  map[string]any `json:"identity"`
			} `json:"impersonatedBy"`
		} `json:"user"`
	}
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.User.Name != "u-target" || entry.User.ImpersonatedBy.Name != "u-admin" || entry.User.ImpersonatedBy.SessionID != "admin-token-id" {
		t.Fatalf("unexpected audit identity: %#v", entry.User)
	}
	if entry.User.ImpersonatedBy.Identity != nil || strings.Contains(output.String(), `"username":"admin"`) {
		t.Fatalf("username feature disabled but human-readable identity was recorded: %s", output.String())
	}
}

func TestAuditMiddlewareRejectsUnrelatedSessions(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}
	otherOriginal := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "other-token-id"}, UserID: "u-other-admin"}
	effective := boundEffectiveToken("impersonation-token", "u-target", otherOriginal)
	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{
		"admin-token":     original,
		"effective-token": effective,
	}}
	h := lifecycleHandler(manager)
	output := &bytes.Buffer{}
	writer, err := audit.NewWriter(output, audit.WriterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	handler := audit.NewAuditLogMiddleware(writer, audit.WithIncludeUsername(func() bool { return true }))(h.AuditMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	req := impersonationRequest(http.MethodGet, "u-target")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unrelated session binding to return 401, got %d", response.Code)
	}
	if strings.Contains(output.String(), `"impersonatedBy"`) {
		t.Fatalf("unexpected impersonator for unrelated sessions: %s", output.String())
	}
}

func TestAuditMiddlewareRejectsBoundSessionWithoutOriginalCookie(t *testing.T) {
	t.Parallel()

	original := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}
	effective := boundEffectiveToken("impersonation-token", "u-target", original)
	manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{"effective-token": effective}}
	h := lifecycleHandler(manager)
	called := false
	handler := h.AuditMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/v3/management.cattle.io.users", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "effective-token"})
	req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: "u-target"}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, req)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing original cookie to return 401, got %d", response.Code)
	}
	if called {
		t.Fatal("request with missing original cookie reached the protected handler")
	}
}

func TestAuditMiddlewareAllowsRecoveryToValidateExpiredSession(t *testing.T) {
	t.Parallel()

	h := lifecycleHandler(&testTokenManager{})
	called := false
	handler := h.AuditMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions/recover", nil)
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "admin-token"})
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "expired-effective-token"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, req)

	if !called {
		t.Fatal("recovery request did not reach its dedicated validator")
	}
}

func TestAuditMiddlewareNeverAttributesInvalidSessions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		original         *apiv3.Token
		effective        *apiv3.Token
		includeOriginal  bool
		includeEffective bool
		requestUser      string
	}{
		{
			name:             "original user binding mismatch",
			original:         &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-other-admin"},
			effective:        boundEffectiveToken("effective-token-id", "u-target", &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "different-token-id"}, UserID: "u-admin"}),
			includeOriginal:  true,
			includeEffective: true,
			requestUser:      "u-target",
		},
		{
			name:             "original token ID mismatch",
			original:         &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
			effective:        boundEffectiveToken("effective-token-id", "u-target", &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "different-token-id"}, UserID: "u-admin"}),
			includeOriginal:  true,
			includeEffective: true,
			requestUser:      "u-target",
		},
		{
			name:             "request user mismatch",
			original:         &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
			effective:        boundEffectiveToken("effective-token-id", "u-target", &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}),
			includeOriginal:  true,
			includeEffective: true,
			requestUser:      "u-other",
		},
		{
			name:             "unmarked effective token",
			original:         &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
			effective:        &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "effective-token-id"}, UserID: "u-target"},
			includeOriginal:  true,
			includeEffective: true,
			requestUser:      "u-target",
		},
		{
			name:             "missing original token",
			effective:        boundEffectiveToken("effective-token-id", "u-target", &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}),
			includeEffective: true,
			requestUser:      "u-target",
		},
		{
			name: "expired original token",
			original: &apiv3.Token{
				ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id", CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute))},
				UserID:     "u-admin",
				TTLMillis:  1,
			},
			effective:        boundEffectiveToken("effective-token-id", "u-target", &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"}),
			includeOriginal:  true,
			includeEffective: true,
			requestUser:      "u-target",
		},
		{
			name:             "expired effective token",
			original:         &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "admin-token-id"}, UserID: "u-admin"},
			effective:        &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "effective-token-id", CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute))}, UserID: "u-target", TTLMillis: 1},
			includeOriginal:  true,
			includeEffective: true,
			requestUser:      "u-target",
		},
		{
			name:        "missing cookies",
			requestUser: "u-target",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manager := &testTokenManager{tokens: map[string]accessor.TokenAccessor{}}
			if test.includeOriginal {
				manager.tokens["admin-token"] = test.original
			}
			if test.includeEffective {
				if test.name == "expired effective token" {
					tokens.BindImpersonationSession(test.effective, test.original)
				}
				manager.tokens["effective-token"] = test.effective
			}
			h := lifecycleHandler(manager)
			output := &bytes.Buffer{}
			writer, err := audit.NewWriter(output, audit.WriterOptions{})
			if err != nil {
				t.Fatal(err)
			}
			handler := audit.NewAuditLogMiddleware(writer, audit.WithIncludeUsername(func() bool { return true }))(h.AuditMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
			req := httptest.NewRequest(http.MethodGet, "/v3/management.cattle.io.users", nil)
			if test.includeOriginal {
				req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "admin-token"})
			}
			if test.includeEffective {
				req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "effective-token"})
			}
			req = req.WithContext(request.WithUser(req.Context(), &requestUser{Name: test.requestUser}))

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if strings.Contains(output.String(), `"impersonatedBy"`) {
				t.Fatalf("invalid session received impersonator attribution: %s", output.String())
			}
		})
	}
}

type requestUser struct {
	Name string
}

func (u *requestUser) GetName() string               { return u.Name }
func (u *requestUser) GetUID() string                { return u.Name }
func (u *requestUser) GetGroups() []string           { return nil }
func (u *requestUser) GetExtra() map[string][]string { return nil }

func boundEffectiveToken(name, userID string, original accessor.TokenAccessor) *apiv3.Token {
	token := &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: name}, UserID: userID}
	tokens.BindImpersonationSession(token, original)
	return token
}

func impersonationRequest(method, userID string) *http.Request {
	req := httptest.NewRequest(method, "/v3/impersonation-sessions/current", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "effective-token"})
	req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "admin-token"})
	return req.WithContext(request.WithUser(req.Context(), &requestUser{Name: userID}))
}

var _ interface {
	GetName() string
	GetUID() string
	GetGroups() []string
	GetExtra() map[string][]string
} = (*requestUser)(nil)

func lifecycleHandler(manager tokenManager) *Handler {
	users := testUsers{
		"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}, PrincipalIDs: []string{"local://admin"}},
		"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, PrincipalIDs: []string{"local://target"}},
	}
	return &Handler{
		users:          func(string) userGetter { return users },
		userAttributes: func(string) userAttributeGetter { return testUserAttributes{} },
		globalRoleBindings: func(string) globalRoleBindingLister {
			return testBindings{items: []apiv3.GlobalRoleBinding{{UserName: "u-admin", GlobalRoleName: "admin"}}}
		},
		globalRoles: func(string) globalRoleGetter {
			return testRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		tokenManager: manager,
	}
}
