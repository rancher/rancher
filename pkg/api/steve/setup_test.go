package steve

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apifakes "github.com/rancher/apiserver/pkg/fakes"
	apitypes "github.com/rancher/apiserver/pkg/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/impersonation"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type steveTestUsers map[string]*apiv3.User

type steveActionTokenManager struct {
	original *apiv3.Token
	created  int
}

func (m *steveActionTokenManager) NewLoginToken(userID string, principal apiv3.Principal, _ []apiv3.Principal, _ string, _ int64, _ string) (*apiv3.Token, string, error) {
	m.created++
	return &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "effective"}, UserID: userID, UserPrincipal: principal}, "secret", nil
}

func (m *steveActionTokenManager) UpdateToken(token *apiv3.Token) (*apiv3.Token, error) {
	return token, nil
}

func (m *steveActionTokenManager) GetToken(value string) (accessor.TokenAccessor, int, error) {
	if value == "original" {
		return m.original, http.StatusOK, nil
	}
	return nil, http.StatusUnauthorized, errors.New("token not found")
}

func (m *steveActionTokenManager) GetTokenForRecovery(string) (*apiv3.Token, bool, error) {
	return nil, false, errors.New("not implemented")
}

func (m *steveActionTokenManager) DeleteTokenByName(string) (int, error) {
	return http.StatusOK, nil
}

func (u steveTestUsers) Get(name string, _ metav1.GetOptions) (*apiv3.User, error) {
	user, ok := u[name]
	if !ok {
		return nil, apierrors.NewNotFound(apiv3.Resource("user"), name)
	}
	return user, nil
}

func TestUserImpersonationActionHandlerUsesCanonicalResponse(t *testing.T) {
	enabled := true
	users := steveTestUsers{
		"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
		"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
	}
	manager := &steveActionTokenManager{original: &apiv3.Token{UserID: "u-admin"}}
	handler := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter { return users },
		func(string) impersonation.UserAttributeGetter { return steveTestAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister {
			return steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
		},
		func(string) impersonation.GlobalRoleGetter {
			return steveTestRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		manager,
	)
	directRequest := httptest.NewRequest(http.MethodPost, "/v3/impersonation-sessions", strings.NewReader(`{"userId":"u-target"}`))
	directRequest.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
	directRequest = directRequest.WithContext(request.WithUser(directRequest.Context(), &userInfo{Name: "u-admin"}))
	directResponse := httptest.NewRecorder()
	handler.Start(directResponse, directRequest)

	req := httptest.NewRequest(http.MethodPost, "/v3/users/u-target", nil)
	req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
	req = req.WithContext(request.WithUser(req.Context(), &userInfo{Name: "u-admin"}))
	response := httptest.NewRecorder()

	userImpersonationActionHandler(handler).ServeHTTP(response, req)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, http.StatusOK, directResponse.Code)
	require.Len(t, response.Result().Cookies(), 2)
	require.Equal(t, "R_IMPERSONATOR_SESS", response.Result().Cookies()[0].Name)
	require.Equal(t, tokens.CookieName, response.Result().Cookies()[1].Name)
	require.Contains(t, response.Body.String(), `"impersonating":true`)
	require.Equal(t, directResponse.Body.String(), response.Body.String())
	require.Equal(t, directResponse.Header().Values("Set-Cookie"), response.Header().Values("Set-Cookie"))
	require.Equal(t, 2, manager.created)
}

func TestUserImpersonationActionHandlerStatusMapping(t *testing.T) {
	enabled := true
	disabled := false
	tests := []struct {
		name     string
		user     *userInfo
		target   *apiv3.User
		nested   bool
		failing  bool
		expected int
	}{
		{name: "unauthenticated", expected: http.StatusUnauthorized},
		{name: "invalid target", user: &userInfo{Name: "u-admin"}, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}}, expected: http.StatusForbidden},
		{name: "nested", user: &userInfo{Name: "u-admin"}, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, nested: true, expected: http.StatusConflict},
		{name: "lookup failure", user: &userInfo{Name: "u-admin"}, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, failing: true, expected: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := steveTestUsers{"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}}}
			if test.target != nil {
				users["u-target"] = test.target
			}
			manager := &steveActionTokenManager{original: &apiv3.Token{UserID: "u-admin"}}
			handler := impersonation.NewHandlerForTest(
				func(string) impersonation.UserGetter { return users },
				func(string) impersonation.UserAttributeGetter { return steveTestAttributes{} },
				func(string) impersonation.GlobalRoleBindingLister {
					if test.failing {
						return steveFailingBindings{}
					}
					return steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
				},
				func(string) impersonation.GlobalRoleGetter {
					return steveTestRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
				},
				manager,
			)
			req := httptest.NewRequest(http.MethodPost, "/v3/users/u-target", nil)
			req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
			if test.nested {
				req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "nested"})
			}
			if test.user != nil {
				req = req.WithContext(request.WithUser(req.Context(), test.user))
			}
			response := httptest.NewRecorder()

			userImpersonationActionHandler(handler).ServeHTTP(response, req)

			require.Equal(t, test.expected, response.Code)
			require.Empty(t, response.Result().Cookies())
			require.Equal(t, 0, manager.created)
		})
	}
}

type steveTestAttributes struct{}

func (steveTestAttributes) Get(string, metav1.GetOptions) (*apiv3.UserAttribute, error) {
	return nil, apierrors.NewNotFound(apiv3.Resource("userattribute"), "test")
}

type steveFailingBindings struct{}

func (steveFailingBindings) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	return nil, errors.New("bindings unavailable")
}

type steveTestAttributeValues map[string]*apiv3.UserAttribute

func (a steveTestAttributeValues) Get(name string, _ metav1.GetOptions) (*apiv3.UserAttribute, error) {
	attribute, ok := a[name]
	if !ok {
		return nil, apierrors.NewNotFound(apiv3.Resource("userattribute"), name)
	}
	return attribute, nil
}

type steveTestBindings []apiv3.GlobalRoleBinding

func (b steveTestBindings) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	return &apiv3.GlobalRoleBindingList{Items: b}, nil
}

type steveTestRoles map[string]*apiv3.GlobalRole

func (r steveTestRoles) Get(name string, _ metav1.GetOptions) (*apiv3.GlobalRole, error) {
	role, ok := r[name]
	if !ok {
		return nil, errors.New("role not found")
	}
	return role, nil
}

func TestUserImpersonationFormatterUsesLifecycleEligibility(t *testing.T) {
	t.Parallel()

	enabled := true
	users := steveTestUsers{
		"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
		"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
		"u-user":   {ObjectMeta: metav1.ObjectMeta{Name: "u-user"}},
	}
	handler := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter { return users },
		func(string) impersonation.UserAttributeGetter { return steveTestAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister {
			return steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
		},
		func(string) impersonation.GlobalRoleGetter {
			return steveTestRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		nil,
	)

	tests := []struct {
		name       string
		caller     string
		wantAction bool
	}{
		{name: "administrator", caller: "u-admin", wantAction: true},
		{name: "non administrator", caller: "u-user", wantAction: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/v3/users/u-target", nil)
			req = req.WithContext(request.WithUser(req.Context(), &userInfo{Name: test.caller}))
			apiRequest := &apitypes.APIRequest{Request: req}
			ctrl := gomock.NewController(t)
			urlBuilder := apifakes.NewMockURLBuilder(ctrl)
			if test.wantAction {
				urlBuilder.EXPECT().Action(gomock.Any(), "u-target", "impersonate").Return("/v3/users/u-target?action=impersonate")
			}
			apiRequest.URLBuilder = urlBuilder
			resource := &apitypes.RawResource{ID: "u-target", Actions: map[string]string{}}

			userImpersonationFormatter(handler)(apiRequest, resource)

			_, gotAction := resource.Actions["impersonate"]
			require.Equal(t, test.wantAction, gotAction)
		})
	}
}

func TestUserImpersonationFormatterDenyMatrix(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false
	tests := []struct {
		name       string
		caller     string
		targetID   string
		target     *apiv3.User
		bindings   steveTestBindings
		anonymous  bool
		nested     bool
		wantAction bool
	}{
		{name: "disabled target", caller: "u-admin", targetID: "u-target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}}, bindings: steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "system target", caller: "u-admin", targetID: "u-target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"system:serviceaccount"}}, bindings: steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "target without principal", caller: "u-admin", targetID: "u-target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled}, bindings: steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "administrator target", caller: "u-admin", targetID: "u-target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}, {UserName: "u-target", GlobalRoleName: "admin"}}},
		{name: "unauthenticated caller", targetID: "u-target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}, anonymous: true},
		{name: "caller lookup error", caller: "u-missing", targetID: "u-target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "target lookup error", caller: "u-admin", targetID: "u-missing", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "nested impersonation", caller: "u-admin", targetID: "u-target", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}, nested: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			users := steveTestUsers{
				"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
			}
			if test.targetID != "u-missing" {
				users[test.targetID] = test.target
			}
			handler := impersonation.NewHandlerForTest(
				func(string) impersonation.UserGetter { return users },
				func(string) impersonation.UserAttributeGetter { return steveTestAttributes{} },
				func(string) impersonation.GlobalRoleBindingLister { return test.bindings },
				func(string) impersonation.GlobalRoleGetter {
					return steveTestRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
				},
				nil,
			)
			req := httptest.NewRequest(http.MethodGet, "/v3/users/"+test.targetID, nil)
			if !test.anonymous {
				req = req.WithContext(request.WithUser(req.Context(), &userInfo{Name: test.caller}))
			}
			if test.nested {
				req.AddCookie(&http.Cookie{Name: "R_IMPERSONATOR_SESS", Value: "nested"})
			}
			apiRequest := &apitypes.APIRequest{Request: req}
			urlBuilder := apifakes.NewMockURLBuilder(gomock.NewController(t))
			if test.wantAction {
				urlBuilder.EXPECT().Action(gomock.Any(), test.targetID, "impersonate").Return("action")
			}
			apiRequest.URLBuilder = urlBuilder
			resource := &apitypes.RawResource{ID: test.targetID, Actions: map[string]string{}}

			userImpersonationFormatter(handler)(apiRequest, resource)

			_, gotAction := resource.Actions["impersonate"]
			require.Equal(t, test.wantAction, gotAction)
		})
	}
}

func TestUserImpersonationFormatterDoesNotShareActionMaps(t *testing.T) {
	t.Parallel()

	enabled := true
	users := steveTestUsers{
		"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
		"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
	}
	handler := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter { return users },
		func(string) impersonation.UserAttributeGetter { return steveTestAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister {
			return steveTestBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
		},
		func(string) impersonation.GlobalRoleGetter {
			return steveTestRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/v3/users/u-target", nil)
	req = req.WithContext(request.WithUser(req.Context(), &userInfo{Name: "u-admin"}))
	urlBuilder := apifakes.NewMockURLBuilder(gomock.NewController(t))
	urlBuilder.EXPECT().Action(gomock.Any(), "u-target", "impersonate").Return("action")
	apiRequest := &apitypes.APIRequest{Request: req, URLBuilder: urlBuilder}
	first := &apitypes.RawResource{ID: "u-target", Actions: map[string]string{}}
	second := &apitypes.RawResource{ID: "u-target", Actions: map[string]string{}}

	userImpersonationFormatter(handler)(apiRequest, first)

	require.Contains(t, first.Actions, "impersonate")
	require.NotContains(t, second.Actions, "impersonate")
}

func TestUserImpersonationFormatterRejectsAllAdminTargetBindingKinds(t *testing.T) {
	t.Parallel()

	enabled := true
	tests := []struct {
		name       string
		binding    apiv3.GlobalRoleBinding
		attributes impersonation.UserAttributeGetter
		role       *apiv3.GlobalRole
	}{
		{
			name:    "custom full access user binding",
			binding: apiv3.GlobalRoleBinding{UserName: "u-target", GlobalRoleName: "custom-admin"},
			role: &apiv3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-admin"},
				Rules: []rbacv1.PolicyRule{
					{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
					{NonResourceURLs: []string{"*"}, Verbs: []string{"*"}},
				},
			},
		},
		{
			name:    "user principal binding",
			binding: apiv3.GlobalRoleBinding{UserPrincipalName: "local://target", GlobalRoleName: "admin"},
			role:    &apiv3.GlobalRole{ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true},
		},
		{
			name:    "group principal binding",
			binding: apiv3.GlobalRoleBinding{GroupPrincipalName: "ldap_group://admins", GlobalRoleName: "admin"},
			attributes: steveTestAttributeValues{"u-target": {
				GroupPrincipals: map[string]apiv3.Principals{
					"ldap": {Items: []apiv3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: "ldap_group://admins"}}}},
				},
			}},
			role: &apiv3.GlobalRole{ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			users := steveTestUsers{
				"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
				"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
			}
			attributes := test.attributes
			if attributes == nil {
				attributes = steveTestAttributes{}
			}
			handler := impersonation.NewHandlerForTest(
				func(string) impersonation.UserGetter { return users },
				func(string) impersonation.UserAttributeGetter { return attributes },
				func(string) impersonation.GlobalRoleBindingLister {
					return steveTestBindings{
						{UserName: "u-admin", GlobalRoleName: "admin"},
						test.binding,
					}
				},
				func(name string) impersonation.GlobalRoleGetter {
					roles := steveTestRoles{
						"admin":        {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true},
						"custom-admin": test.role,
					}
					return roles
				},
				nil,
			)
			req := httptest.NewRequest(http.MethodGet, "/v3/users/u-target", nil)
			req = req.WithContext(request.WithUser(req.Context(), &userInfo{Name: "u-admin"}))
			apiRequest := &apitypes.APIRequest{Request: req}
			apiRequest.URLBuilder = apifakes.NewMockURLBuilder(gomock.NewController(t))
			resource := &apitypes.RawResource{ID: "u-target", Actions: map[string]string{}}

			userImpersonationFormatter(handler)(apiRequest, resource)

			require.NotContains(t, resource.Actions, "impersonate")
		})
	}
}

type userInfo struct {
	Name string
}

func (u *userInfo) GetName() string               { return u.Name }
func (u *userInfo) GetUID() string                { return u.Name }
func (u *userInfo) GetGroups() []string           { return nil }
func (u *userInfo) GetExtra() map[string][]string { return nil }
